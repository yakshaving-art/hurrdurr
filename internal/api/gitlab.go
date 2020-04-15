package api

import (
	"fmt"
	"sync"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"

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

	errs := errors.New()

	users := make(map[string]GitlabUser, 0)
	groups := make(map[string]int, 0)
	projects := make(map[string]int, 0)

	usersCh := make(chan gitlab.User)
	go m.fetchAllUsers(usersCh, &errs)

	adminCount := 0
	for u := range usersCh {
		if u.State == "blocked" {
			logrus.Debugf("appending blocked user %s", u.Username)

			users[u.Username] = GitlabUser{
				ID:             u.ID,
				PrincipalEmail: u.Email,
				Role:           BlockedUserRole,
			}

		} else if u.IsAdmin {
			logrus.Debugf("appending admin %s", u.Username)
			users[u.Username] = GitlabUser{
				ID:             u.ID,
				PrincipalEmail: u.Email,
				Role:           AdminUserRole,
			}
			adminCount++

		} else {
			logrus.Debugf("appending user %s", u.Username)
			// TODO - identify bots
			users[u.Username] = GitlabUser{
				ID:             u.ID,
				PrincipalEmail: u.Email,
				Role:           UserUserRole,
			}
		}
	}

	groupsCh := make(chan gitlab.Group)
	go m.fetchGroups(true, groupsCh, &errs)

	for group := range groupsCh {
		logrus.Debugf("appending group %s", group.FullPath)
		groups[group.FullPath] = group.ID
	}

	projectsCh := make(chan gitlab.Project)
	go m.fetchAllProjects(projectsCh, &errs)

	for project := range projectsCh {
		logrus.Debugf("appending project %s", project.PathWithNamespace)
		projects[project.PathWithNamespace] = project.ID
	}

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

	return errs.ErrorOrNil()
}

// LoadFullGitlabState loads all the state from a remote gitlab instance and returns
// both a querier and a state so they can be used for diffing operations
func LoadFullGitlabState(m GitlabAPIClient) (internal.State, error) {
	groups := make(map[string]internal.Group, 0)
	projects := make(map[string]internal.Project, 0)
	errs := errors.New()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		logrus.Debugf("loading group members...")
		groupsCh := make(chan gitlab.Group)
		go m.fetchGroups(true, groupsCh, &errs)

		for group := range groupsCh {
			members, err := m.fetchGroupMembers(group.FullPath)
			if err != nil {
				errs.Append(err)
				continue
			}

			variables, err := m.fetchGroupVariables(group.FullPath)
			if err != nil {
				errs.Append(err)
				continue
			}

			logrus.Debugf("  appending group '%s' with it's members", group.FullPath)
			groups[group.FullPath] = GitlabGroup{
				fullpath:  group.FullPath,
				members:   members,
				variables: variables,
			}
		}
	}()

	go func() {
		defer wg.Done()

		logrus.Debugf("loading projects...")
		projectsCh := make(chan gitlab.Project)
		go m.fetchAllProjects(projectsCh, &errs)

		for project := range projectsCh {
			groups := make(map[string]internal.Level, 0)

			for _, g := range project.SharedWithGroups {
				group, _, err := m.client.Groups.GetGroup(g.GroupID)
				if err != nil {
					errs.Append(fmt.Errorf("failed to fetch group %s: %s", g.GroupName, err))
					continue
				}
				groups[group.FullPath] = internal.Level(g.GroupAccessLevel)
			}

			members, err := m.fetchProjectMembers(project.PathWithNamespace)
			if err != nil {
				errs.Append(fmt.Errorf("failed to fetch project members for '%s': %s", project.PathWithNamespace, err))
				continue
			}

			// Skip archived projects (they are read-only by definition)
			if project.Archived {
				logrus.Debugf("  skipping variables for project '%s' because it's archived", project.PathWithNamespace)
				continue
			}

			variables := make(map[string]string)

			// Only try to fetch variables from projects with enabled pipelines
			if project.JobsEnabled {
				variables, err = m.fetchProjectVariables(project.PathWithNamespace)
				if err != nil {
					errs.Append(fmt.Errorf("failed to fetch project variables for '%s': %s", project.PathWithNamespace, err))
					continue
				}
			}

			logrus.Debugf("  appending project '%s' with it's members", project.PathWithNamespace)
			projects[project.PathWithNamespace] = GitlabProject{
				fullpath:   project.PathWithNamespace,
				sharedWith: groups,
				members:    members,
				variables:  variables,
			}
		}
	}()

	wg.Wait()

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
	if !ok {
		logrus.Fatalf("could not find user '%s' in the lists of normal, admins, bots or blocked users", username)
	}
	return u.ID
}

// GetGroupID implements the internal querier interface
func (m GitlabQuerier) GetGroupID(group string) int {
	id, ok := m.groups[group]
	if !ok {
		logrus.Fatalf("could not find group '%s' in the list of groups", group)
	}
	return id
}

// IsUser implements Querier interface
func (m GitlabQuerier) IsUser(username string) bool {
	u, ok := m.getUser(username)
	return ok || u.Role == UserUserRole
}

// IsAdmin implements Querier interface
func (m GitlabQuerier) IsAdmin(username string) bool {
	u, ok := m.getUser(username)
	return ok || u.Role == AdminUserRole
}

// IsBlocked implements Querier interface
func (m GitlabQuerier) IsBlocked(username string) bool {
	u, ok := m.getUser(username)
	return ok || u.Role == BlockedUserRole
}

// IsBot implements Querier interface
func (m GitlabQuerier) IsBot(username string) bool {
	u, ok := m.getUser(username)
	return ok || u.Role == BotUserRole
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
	return s.GetUserID(u) != -1 // may be a bot
}

func (s GitlabState) GetUserEmail(username string) (string, bool) {
	return s.Querier.GetUserEmail(username)
}

// GitlabGroup represents a group in a live instance with it's members
//
// This is a helper object that is used to preload the members of a group with
// the state, without leaking gitlab's api structure.
type GitlabGroup struct {
	fullpath  string
	members   map[string]internal.Level
	variables map[string]string
}

// GetFullpath implements the internal.Group interface
func (g GitlabGroup) GetFullpath() string {
	return g.fullpath
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
