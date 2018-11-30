package api

import (
	"fmt"
	"sort"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"

	"github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
)

// GitlabAPIClient is a client for proving high level behaviors when talking to
// a GitLab instance
type GitlabAPIClient struct {
	client  *gitlab.Client
	PerPage int

	querier GitlabQuerier
}

// NewGitlabAPIClient create a new Gitlab API Client
func NewGitlabAPIClient(gitlabToken, gitlabBaseURL string) GitlabAPIClient {
	gitlabClient := gitlab.NewClient(nil, gitlabToken)
	if err := gitlabClient.SetBaseURL(gitlabBaseURL); err != nil {
		logrus.Fatalf("Could not set base URL '%s' to GitLab Client: '%s'", gitlabBaseURL, err)
	}

	return GitlabAPIClient{
		client:  gitlabClient,
		PerPage: 100,
	}
}

// AddMembership implements the APIClient interface
func (m GitlabAPIClient) AddMembership(username, group string, level int) error {
	userID := m.querier.getUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.AddGroupMemberOptions{
		UserID:      &userID,
		AccessLevel: &acl,
	}

	_, _, err := m.client.GroupMembers.AddGroupMember(group, opt)
	if err != nil {
		return fmt.Errorf("failed to add user '%s' to group '%s'", username, group)
	}
	logrus.Infof("added '%s' to '%s' at level '%d'", username, group, level)
	return nil
}

// ChangeMembership implements the APIClient interface
func (m GitlabAPIClient) ChangeMembership(username, group string, level int) error {
	userID := m.querier.getUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.EditGroupMemberOptions{
		AccessLevel: &acl,
	}
	_, _, err := m.client.GroupMembers.EditGroupMember(group, userID, opt)
	if err != nil {
		return fmt.Errorf("failed to change user '%s' in group '%s'", username, group)
	}

	logrus.Infof("changed '%s' in '%s' at level '%d'", username, group, level)
	return nil
}

// RemoveMembership implements the APIClient interface
func (m GitlabAPIClient) RemoveMembership(username, group string) error {
	userID := m.querier.getUserID(username)

	_, err := m.client.GroupMembers.RemoveGroupMember(group, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user '%s' from group '%s'", username, group)
	}
	logrus.Infof(fmt.Sprintf("removed '%s' from '%s'", username, group))
	return nil
}

// LoadState loads all the state from a remote gitlab instance and returns
// both a querier and a state so they can be used for diffing operations
func (m *GitlabAPIClient) LoadState() (internal.Querier, internal.State, error) {
	errs := errors.New()

	querier, err := m.buildQuerier()
	errs.Append(err)

	state, err := m.buildLiveState()
	errs.Append(err)

	m.querier = querier

	return querier, state, errs.ErrorOrNil()
}

func (m GitlabAPIClient) buildQuerier() (GitlabQuerier, error) {
	errs := errors.New()

	logrus.Debugf("building querier...")

	users := make(map[string]int, 0)
	admins := make(map[string]int, 0)

	usersCh := make(chan gitlab.User)
	go m.getUsers(usersCh, &errs)

	for u := range usersCh {
		if u.IsAdmin {
			logrus.Debugf("appending admin %s", u.Username)
			admins[u.Username] = u.ID
		} else {
			logrus.Debugf("appending user %s", u.Username)
			users[u.Username] = u.ID
		}
	}

	groupsCh := make(chan gitlab.Group)
	go m.getGroups(groupsCh, &errs)

	groups := make(map[string]int, 0)

	for group := range groupsCh {
		logrus.Debugf("appending group %s", group.FullPath)
		groups[group.FullPath] = 1
	}

	if len(admins) == 0 {
		errs.Append(fmt.Errorf("no admin was detected, are you using an admin token?"))
	}

	return GitlabQuerier{
		users:  users,
		admins: admins,
		groups: groups,
	}, errs.ErrorOrNil()
}

func (m GitlabAPIClient) buildLiveState() (internal.State, error) {
	errs := errors.New()

	groups := make(map[string]internal.Group, 0)

	groupsCh := make(chan gitlab.Group)
	go m.getGroups(groupsCh, &errs)

	for group := range groupsCh {

		members, err := m.getGroupMembers(group.FullPath)
		if err != nil {
			errs.Append(err)
			continue
		}

		groups[group.FullPath] = GitlabGroup{
			fullpath: group.FullPath,
			members:  members,
		}
	}

	return GitlabState{
		groups: groups,
	}, errs.ErrorOrNil()
}

func (m GitlabAPIClient) getUsers(ch chan gitlab.User, errs *errors.Errors) {
	defer close(ch)
	t := true  // yeah baby... talking about bad interfaces, I need a pointer to true...
	f := false // and another one to false... sadness.

	page := 1
	for {
		opt := &gitlab.ListUsersOptions{
			Active:  &t,
			Blocked: &f,
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

func (m GitlabAPIClient) getGroups(ch chan gitlab.Group, errs *errors.Errors) {
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

func (m GitlabAPIClient) getGroupMembers(fullpath string) (map[string]internal.Level, error) {
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

// GitlabQuerier implements the internal.Querier interface
type GitlabQuerier struct {
	users  map[string]int
	admins map[string]int
	groups map[string]int
}

func (m GitlabQuerier) getUserID(username string) int {
	id, ok := m.users[username]
	if !ok {
		id, ok = m.admins[username]
	}
	if !ok {
		logrus.Fatalf("could not find user '%s' in the lists of users and admins", username)
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

// GroupExists implements Querier interface
func (m GitlabQuerier) GroupExists(g string) bool {
	_, ok := m.groups[g]
	return ok
}

func (m GitlabQuerier) Groups() []string {
	return toStringSlice(m.groups)
}

// Users returns the list of users that are regular users and are not blocked
func (m GitlabQuerier) Users() []string {
	return toStringSlice(m.users)
}

// Admins returns the list of users that are admins and are not blocked
func (m GitlabQuerier) Admins() []string {
	return toStringSlice(m.admins)
}

// GitlabState represents the state of a gitlab instance
type GitlabState struct {
	groups map[string]internal.Group
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

// UnhandledGroups implements internal.State interface
func (s GitlabState) UnhandledGroups() []string {
	return []string{}
}

// GitlabGroup represents a group in a live instance with it's members
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

func toStringSlice(m map[string]int) []string {
	slice := make([]string, 0)
	for v := range m {
		slice = append(slice, v)
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
	return slice
}
