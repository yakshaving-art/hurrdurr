package api

import (
	"fmt"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"

	"github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
)

// GitlabLazyQuerier is a querier that is just too lazy to do things up front
type GitlabLazyQuerier struct {
	api    *GitlabAPIClient
	users  map[string]int
	groups map[string]int
	// projects map[string]int // not yet implemented
}

// LoadPartialGitlabState loads a gitlab state with only the groups and projects that exists
// in the passed configuration
func LoadPartialGitlabState(_ internal.Config, client GitlabAPIClient) (internal.State, error) {
	errs := errors.New()

	groups := make(map[string]internal.Group)
	projects := make(map[string]internal.Project)

	groupsCh := make(chan gitlab.Group)
	go client.fetchGroups(false, groupsCh, &errs)

	for g := range groupsCh {
		members, err := client.fetchGroupMembers(g.FullPath)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch members for group '%s'", err))
			continue
		}

		groups[g.FullPath] = GitlabGroup{
			fullpath: g.FullPath,
			members:  members,
		}
	}

	return GitlabState{
		Querier:  client.Querier,
		groups:   groups,
		projects: projects,
	}, errs.ErrorOrNil()
}

// CreateLazyQuerier creates a gitlab querier that loads the state based in the
// configuration passed in, and then lazily as it is requested.
func CreateLazyQuerier(client *GitlabAPIClient) error {
	errs := errors.New()

	querier := GitlabLazyQuerier{
		api:    client,
		users:  make(map[string]int, 0),
		groups: make(map[string]int, 0),
	}
	client.Querier = querier

	logrus.Debugf("Loading partial groups")
	groupsCh := make(chan gitlab.Group)
	go client.fetchGroups(false, groupsCh, &errs)

	for g := range groupsCh {
		logrus.Debugf("  loading group %s", g.FullPath)
		querier.groups[g.FullPath] = g.ID
	}

	return errs.ErrorOrNil()
}

// GetUserID implements the internal Querier interface
func (g GitlabLazyQuerier) GetUserID(username string) int {
	id, ok := g.users[username]
	if !ok {
		user := g.api.fetchUser(username)
		if user == nil {
			id = -1
		} else {
			id = user.ID
		}
		g.users[username] = id
	}

	return id
}

// GetGroupID implements the internal Querier interface
func (g GitlabLazyQuerier) GetGroupID(fullpath string) int {
	id, ok := g.groups[fullpath]
	if !ok {
		group := g.api.fetchGroup(fullpath)
		if group == nil {
			id = -1
		} else {
			id = group.ID
		}
		g.groups[fullpath] = id
	}
	return id
}

// IsUser implements Querier interface
func (g GitlabLazyQuerier) IsUser(u string) bool {
	return g.GetUserID(u) != -1
}

// IsAdmin implements Querier interface
func (g GitlabLazyQuerier) IsAdmin(_ string) bool {
	return false
}

// IsBlocked implements Querier interface
func (g GitlabLazyQuerier) IsBlocked(_ string) bool {
	return false
}

// GroupExists implements Querier interface
func (g GitlabLazyQuerier) GroupExists(group string) bool {
	return g.GetGroupID(group) != -1
}

// Groups implements Querier interface
func (g GitlabLazyQuerier) Groups() []string {
	return util.ToStringSlice(g.groups)
}

// ProjectExists implements Querier interface
func (g GitlabLazyQuerier) ProjectExists(p string) bool {
	return false
}

// Users returns the list of users that are regular users and are not blocked
func (g GitlabLazyQuerier) Users() []string {
	return util.ToStringSlice(g.users)
}

// Admins returns the list of users that are admins and are not blocked
func (GitlabLazyQuerier) Admins() []string {
	return []string{}
}

// Projects returns the list of projects
func (GitlabLazyQuerier) Projects() []string {
	return []string{} // not implemented yet
}

// Blocked returns the list of users that are blocked
func (GitlabLazyQuerier) Blocked() []string {
	return []string{}
}
