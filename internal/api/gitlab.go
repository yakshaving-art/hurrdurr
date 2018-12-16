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

// GitlabAPIClient is a client for proving high level behaviors when talking to
// a GitLab instance
type GitlabAPIClient struct {
	client    *gitlab.Client
	PerPage   int
	ghostUser string

	Querier internal.Querier
}

// GitlabAPIClientArgs gitlab api client
type GitlabAPIClientArgs struct {
	GitlabToken     string
	GitlabBaseURL   string
	GitlabGhostUser string
}

// NewGitlabAPIClient create a new Gitlab API Client
func NewGitlabAPIClient(args GitlabAPIClientArgs) GitlabAPIClient {
	gitlabClient := gitlab.NewClient(nil, args.GitlabToken)
	if err := gitlabClient.SetBaseURL(args.GitlabBaseURL); err != nil {
		logrus.Fatalf("Could not set base URL '%s' to GitLab Client: '%s'", args.GitlabBaseURL, err)
	}

	return GitlabAPIClient{
		client:    gitlabClient,
		PerPage:   100,
		ghostUser: args.GitlabGhostUser,
	}
}

// CreatePreloadedQuerier creates a Querier with all the data preloaded
func (m *GitlabAPIClient) CreatePreloadedQuerier() error {
	logrus.Debugf("building querier...")

	errs := errors.New()

	users := make(map[string]int, 0)
	admins := make(map[string]int, 0)
	blocked := make(map[string]int, 0)
	groups := make(map[string]int, 0)
	projects := make(map[string]int, 0)

	usersCh := make(chan gitlab.User)
	go m.fetchAllUsers(usersCh, &errs)

	for u := range usersCh {
		if u.State == "blocked" {
			logrus.Debugf("appending blocked user %s", u.Username)
			blocked[u.Username] = u.ID
		} else if u.IsAdmin {
			logrus.Debugf("appending admin %s", u.Username)
			admins[u.Username] = u.ID
		} else {
			logrus.Debugf("appending user %s", u.Username)
			users[u.Username] = u.ID
		}
	}

	groupsCh := make(chan gitlab.Group)
	go m.fetchAllGroups(groupsCh, &errs)

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

	if len(admins) == 0 {
		errs.Append(fmt.Errorf("no admin was detected, are you using an admin token?"))
	}

	m.Querier = GitlabQuerier{
		ghostUser: m.ghostUser,
		users:     users,
		admins:    admins,
		blocked:   blocked,
		groups:    groups,
		projects:  projects,
	}

	return errs.ErrorOrNil()
}

// LoadGitlabState loads all the state from a remote gitlab instance and returns
// both a querier and a state so they can be used for diffing operations
func (m GitlabAPIClient) LoadGitlabState() (internal.State, error) {
	groups := make(map[string]internal.Group, 0)
	projects := make(map[string]internal.Project, 0)
	errs := errors.New()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()

		logrus.Debugf("loading group members...")
		groupsCh := make(chan gitlab.Group)
		go m.fetchAllGroups(groupsCh, &errs)

		for group := range groupsCh {
			members, err := m.fetchGroupMembers(group.FullPath)
			if err != nil {
				errs.Append(err)
				continue
			}

			logrus.Debugf("  appending group '%s' with it's members", group.FullPath)
			groups[group.FullPath] = GitlabGroup{
				fullpath: group.FullPath,
				members:  members,
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
				groups[g.GroupName] = internal.Level(g.GroupAccessLevel)
			}

			logrus.Debugf("  appending project '%s' with it's members", project.PathWithNamespace)
			projects[project.PathWithNamespace] = GitlabProject{
				fullpath:   project.PathWithNamespace,
				sharedWith: groups,
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

// AddGroupMembership implements the APIClient interface
func (m GitlabAPIClient) AddGroupMembership(username, group string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.AddGroupMemberOptions{
		UserID:      &userID,
		AccessLevel: &acl,
	}

	_, _, err := m.client.GroupMembers.AddGroupMember(group, opt)
	if err != nil {
		return fmt.Errorf("failed to add user '%s' to group '%s': %s", username, group, err)
	}
	logrus.Infof("added '%s' to '%s' at level '%d'", username, group, level)
	return nil
}

// ChangeGroupMembership implements the APIClient interface
func (m GitlabAPIClient) ChangeGroupMembership(username, group string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.EditGroupMemberOptions{
		AccessLevel: &acl,
	}
	_, _, err := m.client.GroupMembers.EditGroupMember(group, userID, opt)
	if err != nil {
		return fmt.Errorf("failed to change user '%s' in group '%s': %s", username, group, err)
	}

	logrus.Infof("changed '%s' in '%s' at level '%d'", username, group, level)
	return nil
}

// RemoveGroupMembership implements the APIClient interface
func (m GitlabAPIClient) RemoveGroupMembership(username, group string) error {
	userID := m.Querier.GetUserID(username)

	_, err := m.client.GroupMembers.RemoveGroupMember(group, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user '%s' from group '%s': %s", username, group, err)
	}
	logrus.Infof(fmt.Sprintf("removed '%s' from '%s'", username, group))
	return nil
}

// AddProjectSharing implements the APIClient interface
func (m GitlabAPIClient) AddProjectSharing(project, group string, level internal.Level) error {
	id := m.Querier.GetGroupID(group)
	acl := gitlab.AccessLevelValue(level)

	opt := gitlab.ShareWithGroupOptions{
		GroupID:     &id,
		GroupAccess: &acl,
	}
	_, err := m.client.Projects.ShareProjectWithGroup(project, &opt)
	if err != nil {
		return fmt.Errorf("failed to share project '%s' with group '%s': %s", project, group, err)
	}
	return nil
}

// RemoveProjectSharing implements the APIClient interface
func (m GitlabAPIClient) RemoveProjectSharing(project, group string) error {
	id := m.Querier.GetGroupID(group)

	_, err := m.client.Projects.DeleteSharedProjectFromGroup(project, id)
	if err != nil {
		return fmt.Errorf("failed to remove project '%s' sharing with '%s': %s", project, group, err)
	}

	return nil
}

// AddProjectMembership implements the APIClient interface
func (m GitlabAPIClient) AddProjectMembership(username, project string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.AddProjectMemberOptions{
		UserID:      &userID,
		AccessLevel: &acl,
	}

	_, _, err := m.client.ProjectMembers.AddProjectMember(project, opt)
	if err != nil {
		return fmt.Errorf("failed to add user '%s' to project '%s': %s", username, project, err)
	}
	logrus.Infof("added '%s' to '%s' at level '%d'", username, project, level)
	return nil
}

// ChangeProjectMembership implements the APIClient interface
func (m GitlabAPIClient) ChangeProjectMembership(username, project string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.EditProjectMemberOptions{
		AccessLevel: &acl,
	}
	_, _, err := m.client.ProjectMembers.EditProjectMember(project, userID, opt)
	if err != nil {
		return fmt.Errorf("failed to change user '%s' in project '%s': %s", username, project, err)
	}

	logrus.Infof("changed '%s' in '%s' at level '%d'", username, project, level)
	return nil
}

// RemoveProjectMembership implements the APIClient interface
func (m GitlabAPIClient) RemoveProjectMembership(username, project string) error {
	userID := m.Querier.GetUserID(username)

	_, err := m.client.ProjectMembers.DeleteProjectMember(project, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user '%s' from project '%s': %s", username, project, err)
	}
	logrus.Infof(fmt.Sprintf("removed '%s' from '%s'", username, project))
	return nil
}

// BlockUser implements the APIClient interface
func (m GitlabAPIClient) BlockUser(username string) error {
	userID := m.Querier.GetUserID(username)

	err := m.client.Users.BlockUser(userID)
	if err != nil {
		return fmt.Errorf("failed to block user '%s': %s", username, err)
	}
	logrus.Infof(fmt.Sprintf("blocked user '%s'", username))

	return nil
}

// UnblockUser implements the APIClient interface
func (m GitlabAPIClient) UnblockUser(username string) error {
	userID := m.Querier.GetUserID(username)

	err := m.client.Users.UnblockUser(userID)
	if err != nil {
		return fmt.Errorf("failed to unblock user '%s': %s", username, err)
	}
	logrus.Infof(fmt.Sprintf("unblocked user '%s'", username))

	return nil
}

// SetAdminUser implements the APIClient interface
func (m GitlabAPIClient) SetAdminUser(username string) error {
	userID := m.Querier.GetUserID(username)
	t := true

	_, _, err := m.client.Users.ModifyUser(userID,
		&gitlab.ModifyUserOptions{
			Admin: &t,
		})
	if err != nil {
		return fmt.Errorf("failed to set user '%s' as admin: %s", username, err)
	}
	logrus.Infof(fmt.Sprintf("user '%s' is admin now", username))

	return nil
}

// UnsetAdminUser implements the APIClient interface
func (m GitlabAPIClient) UnsetAdminUser(username string) error {
	userID := m.Querier.GetUserID(username)
	f := false

	_, _, err := m.client.Users.ModifyUser(userID,
		&gitlab.ModifyUserOptions{
			Admin: &f,
		})
	if err != nil {
		return fmt.Errorf("failed to unset user '%s' as admin: %s", username, err)
	}
	logrus.Infof(fmt.Sprintf("user '%s' is not admin anymore", username))

	return nil
}

func (m GitlabAPIClient) fetchAllUsers(ch chan gitlab.User, errs *errors.Errors) {
	defer close(ch)

	page := 1
	for {
		opt := &gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}
		users, resp, err := m.client.Users.ListUsers(opt)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch users: %s", err))
			break
		}

		for _, user := range users {
			ch <- *user
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
}

func (m GitlabAPIClient) fetchUser(username string) *gitlab.User {
	users, _, err := m.client.Users.ListUsers(&gitlab.ListUsersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
			Page:    1,
		},
		Username: &username,
	})
	if err != nil {
		logrus.Fatalf("failed to fetch user '%s': %s", username, err)
	}

	if len(users) == 0 {
		return nil
	}
	return users[0]
}

func (m GitlabAPIClient) fetchGroup(fullpath string) *gitlab.Group {
	group, _, err := m.client.Groups.GetGroup(fullpath)
	if err != nil {
		logrus.Fatalf("failed to fetch group '%s': %s", fullpath, err)
	}

	return group
}

func (m GitlabAPIClient) fetchAllGroups(ch chan gitlab.Group, errs *errors.Errors) {
	defer close(ch)
	t := true // yeah baby... talking about bad interfaces, I need a pointer to true...

	page := 1
	for {
		opt := &gitlab.ListGroupsOptions{
			AllAvailable: &t,
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}

		groups, resp, err := m.client.Groups.ListGroups(opt)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch groups: %s", err))
			break
		}

		for _, group := range groups {
			ch <- *group
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
}

func (m GitlabAPIClient) fetchGroupMembers(fullpath string) (map[string]internal.Level, error) {
	groupMembers := make(map[string]internal.Level)

	page := 1
	for {
		opt := &gitlab.ListGroupMembersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}

		members, resp, err := m.client.Groups.ListGroupMembers(fullpath, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch group members for %s: %s", fullpath, err)
		}

		for _, member := range members {
			groupMembers[member.Username] = internal.Level(member.AccessLevel)
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
	return groupMembers, nil
}

func (m GitlabAPIClient) fetchAllProjects(ch chan gitlab.Project, errs *errors.Errors) {
	defer close(ch)

	page := 1
	for {
		opt := &gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}

		prjs, resp, err := m.client.Projects.ListProjects(opt)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch the list of projects: %s", err))
			return
		}

		for _, p := range prjs {
			ch <- *p
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
}

// GitlabQuerier implements the internal.Querier interface
type GitlabQuerier struct {
	ghostUser string
	users     map[string]int
	admins    map[string]int
	blocked   map[string]int
	groups    map[string]int
	projects  map[string]int
}

// GetUserID implements the internal querier interface
func (m GitlabQuerier) GetUserID(username string) int {
	id, ok := m.users[username]
	if !ok {
		id, ok = m.admins[username]
	}
	if !ok {
		id, ok = m.blocked[username]
	}
	if !ok {
		logrus.Fatalf("could not find user '%s' in the lists of normal, admins or blocked users", username)
	}
	return id
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
func (m GitlabQuerier) IsUser(u string) bool {
	_, ok := m.users[u]
	return ok
}

// IsAdmin implements Querier interface
func (m GitlabQuerier) IsAdmin(u string) bool {
	_, ok := m.admins[u]
	return ok
}

// IsBlocked implements Querier interface
func (m GitlabQuerier) IsBlocked(u string) bool {
	_, ok := m.blocked[u]
	return ok
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
	return util.ToStringSliceIgnoring(m.users, m.ghostUser)
}

// Admins returns the list of users that are admins and are not blocked
func (m GitlabQuerier) Admins() []string {
	return util.ToStringSliceIgnoring(m.admins, m.ghostUser)
}

// Blocked returns the list of users that are blocked
func (m GitlabQuerier) Blocked() []string {
	return util.ToStringSlice(m.blocked)
}

// Projects returns the list of projects
func (m GitlabQuerier) Projects() []string {
	return util.ToStringSlice(m.projects)
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
func (s GitlabState) UnhandledGroups() []string {
	return []string{}
}

// GitlabGroup represents a group in a live instance with it's members
//
// This is a helper object that is used to preload the members of a group with
// the state, without leaking gitlab's api structure.
type GitlabGroup struct {
	fullpath string
	members  map[string]internal.Level
}

// GetFullpath implements the internal.Group interface
func (g GitlabGroup) GetFullpath() string {
	return g.fullpath
}

// GetMembers implements the internal.Group interface
func (g GitlabGroup) GetMembers() map[string]internal.Level {
	return g.members
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
