package api

import (
	"fmt"
	"sync"
	"time"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"
	"gitlab.com/yakshaving.art/hurrdurr/pkg/workerpool"

	"github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
)

// Roles
var (
	AdminUserRole   = "ADMIN"
	UserUserRole    = "USER"
	BotUserRole     = "BOT"
	BlockedUserRole = "BLOCKED"
)

// CreatePreloadedQuerier creates a Querier with all the data preloaded
func CreatePreloadedQuerier(m *GitlabAPIClient) error {
	logrus.Debugf("building querier...")
	querierStartTime := time.Now()
	errs := errors.New()

	users := make(map[string]GitlabUser)
	groups := make(map[string]int)
	projects := make(map[string]int)
	usersCh := make(chan gitlab.User)

	go m.fetchAllUsers(usersCh, &errs)

	logrus.Debugf("populating users map...")
	startTime := time.Now()
	adminCount := 0
	for u := range usersCh {
		if u.State == "blocked" {
			users[u.Username] = GitlabUser{
				ID:             u.ID,
				PrincipalEmail: u.Email,
				Role:           BlockedUserRole,
			}

		} else if u.IsAdmin {
			users[u.Username] = GitlabUser{
				ID:             u.ID,
				PrincipalEmail: u.Email,
				Role:           AdminUserRole,
			}
			adminCount++

		} else {
			// TODO - identify bots
			users[u.Username] = GitlabUser{
				ID:             u.ID,
				PrincipalEmail: u.Email,
				Role:           UserUserRole,
			}
			// logrus.Debugf("appending user %s (took %s)", u.Username, time.Since(startTime))
		}
	}
	logrus.Debugf("done populating users map (took %s)", time.Since(startTime))

	groupsCh := make(chan gitlab.Group)

	go m.fetchGroups(true, groupsCh, &errs)

	logrus.Debugf("populating groups map...")
	startTime = time.Now()
	for group := range groupsCh {
		groups[group.FullPath] = group.ID
	}
	logrus.Debugf("done populating groups map (took %s)", time.Since(startTime))

	projectsCh := make(chan gitlab.Project)
	go m.fetchAllProjects(projectsCh, &errs)
	logrus.Debugf("populating projects map...")
	startTime = time.Now()
	for project := range projectsCh {
		projects[project.PathWithNamespace] = project.ID
	}
	logrus.Debugf("done populating projects map (took %s)", time.Since(startTime))

	if adminCount == 0 {
		errs.Append(fmt.Errorf("no admin was detected, are you using an admin token?"))
	}

	m.Querier = GitlabQuerier{
		ghostUser:   m.ghostUser,
		currentUser: m.CurrentUser(),
		users:       users,
		groups:      groups,
		projects:    projects,
	}

	logrus.Debugf("done building querier (took %s)", time.Since(querierStartTime))
	return errs.ErrorOrNil()
}

// LoadFullGitlabState loads all the state from a remote gitlab instance and returns
// both a querier and a state so they can be used for diffing operations
func LoadFullGitlabState(m GitlabAPIClient) (internal.State, error) {
	groups := make(map[string]internal.Group, m.Concurrency)
	projects := make(map[string]internal.Project, m.Concurrency)

	errs := errors.New()

	groupsLock := &sync.Mutex{}
	projectsLock := &sync.Mutex{}

	wg := &sync.WaitGroup{}

	// Create a worker pool that controls concurrency tightly, with as many slots are concurrency is enabled
	workers := workerpool.New(m.Concurrency)

	logrus.Infof("loading group members and project details...")

	globalTime := time.Now()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logrus.Debugf("loading group members with a concurrency of %d...", m.Concurrency)

		groupsCh := make(chan gitlab.Group)

		go m.fetchGroups(true, groupsCh, &errs)

		for group := range groupsCh {

			wg.Add(1) // for every group, wait for it to complete
			workers.Do(func(group gitlab.Group) func() {
				return func() {
					defer wg.Done()

					jobTime := time.Now()

					sharedGroups := make(map[string]internal.Level, 0)
					for _, sg := range group.SharedWithGroups {
						g, _, err := m.client.Groups.GetGroup(sg.GroupID)
						if err != nil {
							errs.Append(fmt.Errorf("failed to fetch group %s: %s", sg.GroupName, err))
							return
						}
						sharedGroups[g.FullPath] = internal.Level(sg.GroupAccessLevel)
					}

					members, err := m.fetchGroupMembers(group.FullPath)
					if err != nil {
						errs.Append(fmt.Errorf("failed fetching group members (took %s): %s", time.Since(jobTime), err))
						return
					}

					variables, err := m.fetchGroupVariables(group.FullPath)
					if err != nil {
						errs.Append(fmt.Errorf("failed fetching group variables (took %s): %s", time.Since(jobTime), err))
						return
					}

					groupsLock.Lock()
					defer groupsLock.Unlock()

					groups[group.FullPath] = GitlabGroup{
						fullpath:   group.FullPath,
						sharedWith: sharedGroups,
						members:    members,
						variables:  variables,
					}
					logrus.Debugf("done fetching group %q variables and members (took %s)", group.FullPath, time.Since(jobTime))
				}
			}(group))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		logrus.Debugf("loading projects with a concurrency of %d...", m.Concurrency)

		projectsCh := make(chan gitlab.Project)

		go m.fetchAllProjects(projectsCh, &errs)

		for project := range projectsCh {

			wg.Add(1) // for every project, wait for it to complete
			workers.Do(func(project gitlab.Project) func() {
				return func() {
					defer wg.Done()

					jobTime := time.Now()
					groups := make(map[string]internal.Level)
					for _, g := range project.SharedWithGroups {
						group, _, err := m.client.Groups.GetGroup(g.GroupID)
						if err != nil {
							errs.Append(fmt.Errorf("failed to fetch group %s (took %s): %s", g.GroupName, time.Since(jobTime), err))
							return
						}
						groups[group.FullPath] = internal.Level(g.GroupAccessLevel)
					}

					members, err := m.fetchProjectMembers(project.PathWithNamespace)
					if err != nil {
						errs.Append(fmt.Errorf("failed to fetch project members for '%s' (took %s): %s", project.PathWithNamespace, time.Since(jobTime), err))
						return
					}

					variables := make(map[string]string)

					// Only try to fetch variables from projects with enabled pipelines
					// Skip archived projects (they are read-only by definition)
					if project.JobsEnabled && !project.Archived {
						variables, err = m.fetchProjectVariables(project.PathWithNamespace)
						if err != nil {
							errs.Append(fmt.Errorf("failed to fetch project variables for '%s' (took %s): %s", project.PathWithNamespace, time.Since(jobTime), err))
							return
						}
					}

					logrus.Tracef("appending project '%s' with its members (took %s)", project.PathWithNamespace, time.Since(jobTime))

					projectsLock.Lock()
					defer projectsLock.Unlock()

					projects[project.PathWithNamespace] = GitlabProject{
						fullpath:   project.PathWithNamespace,
						sharedWith: groups,
						members:    members,
						variables:  variables,
					}

					logrus.Debugf("done loading project %q (took %s)", project.PathWithNamespace, time.Since(jobTime))
				}
			}(project))
		}

	}()

	logrus.Debugf("workpool initialized, waiting on executing all the jobs...")

	wg.Wait()

	workers.Wait() // We shouldn't be waiting for anything, but just to be safe

	logrus.Infof("done loading group members and project details (took %s)", time.Since(globalTime))

	return GitlabState{
		Querier:  m.Querier,
		groups:   groups,
		projects: projects,
	}, errs.ErrorOrNil()
}

// GitlabUser is a user in gitlab, which has a role like admin, user, blocked or bot
type GitlabUser struct {
	Role           string
	ID             int
	PrincipalEmail string
}

// GitlabQuerier implements the internal.Querier interface
type GitlabQuerier struct {
	ghostUser   string
	currentUser string
	users       map[string]GitlabUser
	// users       map[string]int
	// admins      map[string]int
	// blocked     map[string]int
	groups   map[string]int
	projects map[string]int
}

func (m GitlabQuerier) getUser(username string) (GitlabUser, bool) {
	u, ok := m.users[username]
	return u, ok
}

// GetUserID implements the internal querier interface
func (m GitlabQuerier) GetUserID(username string) int {
	u, ok := m.getUser(username)
	if ok {
		return u.ID
	}

	logrus.Debugf("could not find user '%s' in the lists of normal, admins, bots or blocked users", username)
	return -1 //signals that the user does not exists
}

// GetGroupID implements the internal querier interface
func (m GitlabQuerier) GetGroupID(group string) int {
	id, ok := m.groups[group]
	if ok {
		return id
	}
	logrus.Debugf("could not find group '%s' in the list of groups", group)
	return -1 //signals that the group does not exists
}

// IsUser implements Querier interface
func (m GitlabQuerier) IsUser(username string) bool {
	u, ok := m.getUser(username)
	logrus.Debugf("checking user '%s' being a user, role '%s', exists '%t'", username, u.Role, ok)
	return ok && u.Role == UserUserRole
}

// IsAdmin implements Querier interface
func (m GitlabQuerier) IsAdmin(username string) bool {
	u, ok := m.getUser(username)
	logrus.Debugf("checking user '%s' for being admin, role '%s', exists '%t'", username, u.Role, ok)
	return ok && u.Role == AdminUserRole
}

// IsBlocked implements Querier interface
func (m GitlabQuerier) IsBlocked(username string) bool {
	logrus.Debugf("Checking if user '%s' is blocked", username)
	u, ok := m.getUser(username)

	logrus.Debugf("checking user '%s' for being blocked, role '%s', exists '%t'", username, u.Role, ok)
	if !ok {
		return true // a non existing user can be considered as blocked
	}

	return u.Role == BlockedUserRole
}

// IsBot implements Querier interface
func (m GitlabQuerier) IsBot(username string) bool {
	u, ok := m.getUser(username)
	logrus.Debugf("checking user '%s' being a bot, role '%s', exists '%t'", username, u.Role, ok)
	return ok && u.Role == BotUserRole
}

// GetUserEmail implements Querier interface
func (m GitlabQuerier) GetUserEmail(username string) (string, bool) {
	u, ok := m.getUser(username)
	if !ok {
		return "", false
	}
	return u.PrincipalEmail, true
}

// GroupExists implements Querier interface
func (m GitlabQuerier) GroupExists(g string) bool {
	_, ok := m.groups[g]
	return ok
}

// Groups implements Querier interface
func (m GitlabQuerier) Groups() []string {
	return util.ToStringSlice(m.groups)
}

// ProjectExists implements Querier interface
func (m GitlabQuerier) ProjectExists(p string) bool {
	_, ok := m.projects[p]
	return ok
}

// Users returns the list of users that are regular users and are not blocked
func (m GitlabQuerier) Users() []string {
	return m.filterUsers(UserUserRole, m.ghostUser)
}

// Admins returns the list of users that are admins and are not blocked
func (m GitlabQuerier) Admins() []string {
	return m.filterUsers(AdminUserRole, m.ghostUser)
}

// Blocked returns the list of users that are blocked
func (m GitlabQuerier) Blocked() []string {
	return m.filterUsers(BlockedUserRole, "")
}

func (m GitlabQuerier) filterUsers(filterForRole, ignoreUser string) []string {
	users := make([]string, 0)

	for username, u := range m.users {
		if username == ignoreUser {
			continue
		}
		if u.Role != filterForRole {
			continue
		}
		users = append(users, username)
	}
	return users

}

// Projects returns the list of projects
func (m GitlabQuerier) Projects() []string {
	return util.ToStringSlice(m.projects)
}

// CurrentUser returns the current user talking to the API
func (m GitlabQuerier) CurrentUser() string {
	return m.currentUser
}

// GitlabState represents the state of a gitlab instance
//
// This object is used to calculate the diff of state between a current and a
// desired state. This particular kind of GitlabState will preload all the data
// to optimize for performance at he expense of keeping everything in memory.
// This is not particularly bad in a small instance, but it will take "a lot" to
// load gitlab.com state.
type GitlabState struct {
	internal.Querier
	groups   map[string]internal.Group
	projects map[string]internal.Project
}

// Groups implements internal.State interface
func (s GitlabState) Groups() []internal.Group {
	groups := make([]internal.Group, 0)
	for _, g := range s.groups {
		groups = append(groups, g)
	}
	return groups
}

// Group implements internal.State interface
func (s GitlabState) Group(name string) (internal.Group, bool) {
	g, ok := s.groups[name]
	return g, ok
}

// Project implements internal.State interface
func (s GitlabState) Project(fullpath string) (internal.Project, bool) {
	p, ok := s.projects[fullpath]
	return p, ok
}

// Projects implements internal.State interface
func (s GitlabState) Projects() []internal.Project {
	projects := make([]internal.Project, 0)
	for _, p := range s.projects {
		projects = append(projects, p)
	}
	return projects
}

// UnhandledGroups implements internal.State interface
func (GitlabState) UnhandledGroups() []string {
	return []string{}
}

// BotUsers returns an empty list of bots
func (GitlabState) BotUsers() map[string]string {
	return make(map[string]string)
}

// IsBot implements Querier interface
func (s GitlabState) IsBot(u string) bool {
	return s.Querier.GetUserID(u) != -1
}

// GetUserEmail implements the querier interface
func (s GitlabState) GetUserEmail(username string) (string, bool) {
	return s.Querier.GetUserEmail(username)
}

// GitlabGroup represents a group in a live instance with it's members
//
// This is a helper object that is used to preload the members of a group with
// the state, without leaking gitlab's api structure.
type GitlabGroup struct {
	fullpath   string
	sharedWith map[string]internal.Level
	members    map[string]internal.Level
	variables  map[string]string
}

// GetFullpath implements the internal.Group interface
func (g GitlabGroup) GetFullpath() string {
	return g.fullpath
}

// GetSharedGroups implements internal.Group interface
func (g GitlabGroup) GetSharedGroups() map[string]internal.Level {
	return g.sharedWith
}

// GetGroupLevel implements internal.Group interface
func (g GitlabGroup) GetGroupLevel(group string) (internal.Level, bool) {
	level, ok := g.sharedWith[group]
	return level, ok
}

// GetMembers implements the internal.Group interface
func (g GitlabGroup) GetMembers() map[string]internal.Level {
	return g.members
}

// HasVariable implements internal.HasVariable interface
func (g GitlabGroup) HasVariable(key string) bool {
	_, ok := g.variables[key]
	return ok
}

// VariableEquals implements internal.VariableEquals interface
func (g GitlabGroup) VariableEquals(key, value string) bool {
	if current, ok := g.variables[key]; ok {
		return current == value
	}
	return false
}

// GetVariables implements internal.GetVariables interface
func (g GitlabGroup) GetVariables() map[string]string {
	return g.variables
}

// GitlabProject implements internal.Project interface
//
// This is a helper object that is used to load a project with the list of
// groups it's shared with, and the specific members that it has assigned.
// It exists to prevent leaking gitlab's API.
type GitlabProject struct {
	fullpath   string
	sharedWith map[string]internal.Level
	members    map[string]internal.Level
	variables  map[string]string
}

// GetFullpath implements internal.Project interface
func (g GitlabProject) GetFullpath() string {
	return g.fullpath
}

// GetSharedGroups implements internal.Project interface
func (g GitlabProject) GetSharedGroups() map[string]internal.Level {
	return g.sharedWith
}

// GetGroupLevel implements internal.Project interface
func (g GitlabProject) GetGroupLevel(group string) (internal.Level, bool) {
	level, ok := g.sharedWith[group]
	return level, ok
}

// GetMembers implements internal.Project interface
func (g GitlabProject) GetMembers() map[string]internal.Level {
	return g.members
}

// HasVariable implements internal.HasVariable interface
func (g GitlabProject) HasVariable(key string) bool {
	_, ok := g.variables[key]
	return ok
}

// VariableEquals implements internal.VariableEquals interface
func (g GitlabProject) VariableEquals(key, value string) bool {
	if current, ok := g.variables[key]; ok {
		return current == value
	}
	return false
}

// GetVariables implements internal.GetVariables interface
func (g GitlabProject) GetVariables() map[string]string {
	return g.variables
}

func b(bb bool) *bool {
	return &bb
}
