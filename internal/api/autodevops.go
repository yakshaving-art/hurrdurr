package api

import (
	"fmt"
	"sort"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"

	"github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
)

// GitlabLazyQuerier is a querier that is just too lazy to do things up front
type GitlabLazyQuerier struct {
	api      *GitlabAPIClient
	users    map[string]GitlabUser
	groups   map[string]int
	projects map[string]int // not yet implemented
}

// LoadPartialGitlabState loads a gitlab state with only the groups and projects that exists
// in the passed configuration
func LoadPartialGitlabState(cnf internal.Config, client GitlabAPIClient) (internal.State, error) {
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

		vars, err := client.fetchGroupVariables(g.FullPath)
		if err != nil {
			if err == ErrForbiddenAction {
				logrus.Debugf("User is not allowed to read group variables from %s", g.FullPath)
			} else {
				errs.Append(fmt.Errorf("failed to fetch variables for group '%s': %s", g.FullPath, err))
				continue
			}
		}

		groups[g.FullPath] = GitlabGroup{
			fullpath:  g.FullPath,
			members:   members,
			variables: vars,
		}
	}

	for p := range cnf.Projects {
		project, err := client.fetchProject(p)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch project: %s", p))
			continue
		}
		if project == nil {
			errs.Append(fmt.Errorf("project '%s' does not exist", p))
			continue
		}

		members, err := client.fetchProjectMembers(p)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch project members for '%s': %s", project.PathWithNamespace, err))
			continue
		}

		vars, err := client.fetchProjectVariables(p)
		if err != nil {
			if err == ErrForbiddenAction {
				logrus.Debugf("User is not allowed to read project variables from %s", project.PathWithNamespace)
			} else {
				errs.Append(fmt.Errorf("failed to fetch project variables for '%s': %s", project.PathWithNamespace, err))
				continue
			}
		}

		groups := make(map[string]internal.Level, 0)

		for _, g := range project.SharedWithGroups {
			groups[g.GroupName] = internal.Level(g.GroupAccessLevel)
		}

		projects[p] = GitlabProject{
			fullpath:   p,
			sharedWith: groups,
			members:    members,
			variables:  vars,
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
		api:      client,
		users:    make(map[string]GitlabUser, 0),
		groups:   make(map[string]int, 0),
		projects: make(map[string]int, 0),
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
	u, ok := g.users[username]
	if !ok {
		user := g.api.fetchUser(username)
		if user == nil {
			u = GitlabUser{
				ID: -1,
			}
		} else {
			u = GitlabUser{
				ID:             user.ID,
				PrincipalEmail: user.Email,
				Role:           UserUserRole,
			}
		}
		g.users[username] = u
	}

	return u.ID
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

// ProjectExists implements Querier interface
func (g GitlabLazyQuerier) ProjectExists(fullpath string) bool {
	id, ok := g.projects[fullpath]
	if !ok {
		project, err := g.api.fetchProject(fullpath)
		if err != nil {
			logrus.Fatalf("failed to fetch project '%s': %s", fullpath, err)
		}

		if project == nil {
			id = -1
		} else {
			id = project.ID
		}
		g.projects[fullpath] = id
	}
	return id != -1
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

// Users returns the list of users that are regular users and are not blocked
func (g GitlabLazyQuerier) Users() []string {
	users := make([]string, 0)
	for u := range g.users {
		users = append(users, u)
	}
	sort.Strings(users)
	return users
}

// Admins returns the list of users that are admins and are not blocked
func (GitlabLazyQuerier) Admins() []string {
	return []string{}
}

// Projects returns the list of projects
func (GitlabLazyQuerier) Projects() []string {
	return []string{}
}

// Blocked returns the list of users that are blocked
func (GitlabLazyQuerier) Blocked() []string {
	return []string{}
}

// CurrentUser returns the current user talking to the API
func (g GitlabLazyQuerier) CurrentUser() string {
	return g.api.CurrentUser()
}

// GetUserEmail returns an empty string and false
func (g GitlabLazyQuerier) GetUserEmail(username string) (string, bool) {
	return "", false
}
