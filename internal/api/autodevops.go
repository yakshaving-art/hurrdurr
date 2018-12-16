package api

import (
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"
)

// GitlabLazyQuerier is a querier that is just too lazy to do things up front
type GitlabLazyQuerier struct {
	api    GitlabAPIClient
	users  map[string]int
	groups map[string]int
	// projects map[string]int // not yet implemented
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
func (g GitlabLazyQuerier) IsAdmin(u string) bool {
	return false
}

// IsBlocked implements Querier interface
func (g GitlabLazyQuerier) IsBlocked(u string) bool {
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

// Blocked returns the list of users that are blocked
func (GitlabLazyQuerier) Blocked() []string {
	return []string{}
}
