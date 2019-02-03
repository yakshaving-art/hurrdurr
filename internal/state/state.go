package state

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"

	"github.com/sirupsen/logrus"
)

// LocalGroup represents a group with a fullpath and it's members that is loaded from a yaml file
type LocalGroup struct {
	Fullpath string
	Members  map[string]internal.Level
	Subquery bool

	Variables map[string]string
}

// GetFullpath implements Group interface
func (g LocalGroup) GetFullpath() string {
	return g.Fullpath
}

// GetMembers implements Group interface
func (g LocalGroup) GetMembers() map[string]internal.Level {
	return g.Members
}

// HasSubquery implements Group interface
func (g LocalGroup) HasSubquery() bool {
	return g.Subquery
}

// HasVariable implements Group interface
func (g LocalGroup) HasVariable(key string) bool {
	_, ok := g.Variables[key]
	return ok
}

// VariableEquals implements Group interface
func (g LocalGroup) VariableEquals(key, value string) bool {
	if v, ok := g.Variables[key]; ok {
		return value == v
	}
	return false
}

// GetVariables implements Group interface
func (g LocalGroup) GetVariables() map[string]string {
	return g.Variables
}

func (g LocalGroup) addMember(username string, level internal.Level) {
	l, ok := g.Members[username]
	if ok && l > level {
		return
	}
	g.Members[username] = level
}

func (g LocalGroup) String() string {
	return g.GetFullpath()
}

func (g *LocalGroup) setHasSubquery(b bool) {
	g.Subquery = b
}

// LocalProject is a local implementation of projects loaded from a file
type LocalProject struct {
	Fullpath     string
	SharedGroups map[string]internal.Level
	Members      map[string]internal.Level

	Variables map[string]string
}

// GetFullpath implements internal.Project interface
func (l LocalProject) GetFullpath() string {
	return l.Fullpath
}

// GetSharedGroups implements internal.Project interface
func (l LocalProject) GetSharedGroups() map[string]internal.Level {
	return l.SharedGroups
}

// GetGroupLevel implements internal.Project interface
func (l LocalProject) GetGroupLevel(group string) (internal.Level, bool) {
	level, ok := l.SharedGroups[group]
	return level, ok
}

// HasVariable implements Group interface
func (l LocalProject) HasVariable(key string) bool {
	_, ok := l.Variables[key]
	return ok
}

// VariableEquals implements Group interface
func (l LocalProject) VariableEquals(key, value string) bool {
	if v, ok := l.Variables[key]; ok {
		return value == v
	}
	return false
}

// GetVariables implements Project interface
func (l LocalProject) GetVariables() map[string]string {
	return l.Variables
}

func (l *LocalProject) addGroupSharing(group string, level internal.Level) {
	l.SharedGroups[group] = level
}

// GetMembers implements internal.Project interface
func (l LocalProject) GetMembers() map[string]internal.Level {
	return l.Members
}

func (l LocalProject) String() string {
	return l.GetFullpath()
}

func (l LocalProject) addMember(username string, level internal.Level) {
	lvl, ok := l.Members[username]
	if ok && lvl > level {
		return
	}
	l.Members[username] = level
}

// LoadStateFromFile loads the desired state from a file
func LoadStateFromFile(c internal.Config, q internal.Querier) (internal.State, error) {
	l, err := configToLocalState(c, q)
	if err != nil {
		return nil, fmt.Errorf("failed to build local state: %s", err)
	}

	return l, nil
}

type localState struct {
	currentUser     string
	groups          map[string]*LocalGroup
	projects        map[string]*LocalProject
	admins          map[string]int
	blocked         map[string]int
	unhandledGroups []string
}

func (s localState) addGroup(g *LocalGroup) {
	s.groups[g.Fullpath] = g
}

func (s localState) Groups() []internal.Group {
	groups := make([]internal.Group, 0)
	for _, g := range s.groups {
		groups = append(groups, *g)
	}
	return groups
}

func (s localState) Group(name string) (internal.Group, bool) {
	g, ok := s.groups[name]
	return g, ok
}

func (s localState) UnhandledGroups() []string {
	return s.unhandledGroups
}

func (s localState) addProject(p *LocalProject) {
	s.projects[p.GetFullpath()] = p
}

func (s localState) Projects() []internal.Project {
	projects := make([]internal.Project, 0)
	for _, p := range s.projects {
		projects = append(projects, *p)
	}
	return projects
}

func (s localState) Project(fullpath string) (internal.Project, bool) {
	p, ok := s.projects[fullpath]
	return p, ok
}

func (s localState) IsAdmin(username string) bool {
	_, ok := s.admins[username]
	return ok
}

func (s localState) IsBlocked(username string) bool {
	_, ok := s.blocked[username]
	return ok
}

func (s localState) CurrentUser() string {
	return s.currentUser
}

// Admins implements State interface
func (s localState) Admins() []string {
	return util.ToStringSlice(s.admins)
}

// Blocked implements State interface
func (s localState) Blocked() []string {
	return util.ToStringSlice(s.blocked)
}

func configToLocalState(c internal.Config, q internal.Querier) (localState, error) {
	l := localState{
		currentUser: q.CurrentUser(),
		groups:      make(map[string]*LocalGroup, 0),
		projects:    make(map[string]*LocalProject, 0),
		admins:      make(map[string]int, 0),
		blocked:     make(map[string]int, 0),
	}

	errs := errors.New() // This object aggregates all the errors to dump them all at the end
	queries := make([]query, 0)

	for fullpath, g := range c.Groups {
		if !q.GroupExists(fullpath) {
			errs.Append(fmt.Errorf("Group '%s' does not exist", fullpath))
			continue
		}

		group := &LocalGroup{
			Fullpath: fullpath,
			Members:  make(map[string]internal.Level, 0),

			Variables: make(map[string]string, 0),
		}

		for k, envKey := range g.Variables {
			value, ok := os.LookupEnv(envKey)
			if !ok {
				errs.Append(fmt.Errorf("Group contains secret '%s'='%s' which is not loaded in the environment", k, envKey))
				continue
			}
			group.Variables[k] = value
		}

		addMembers := func(members []string, level internal.Level) {
			for _, member := range members {
				if strings.HasPrefix(member, "query:") {
					queries = append(queries, query{
						query:       strings.TrimSpace(member[6:]),
						level:       level,
						memberAdder: group,
					})
					group.setHasSubquery(true)
					continue
				}
				if q.IsBlocked(member) {
					errs.Append(fmt.Errorf("User '%s' is blocked, it should not be included in group '%s'", member, fullpath))
					continue
				}
				if !q.IsUser(member) && !q.IsAdmin(member) {
					errs.Append(fmt.Errorf("User '%s' does not exists for group '%s'", member, fullpath))
					continue
				}

				group.addMember(member, level)
			}
		}

		addMembers(g.Guests, internal.Guest)
		addMembers(g.Reporters, internal.Reporter)
		addMembers(g.Developers, internal.Developer)
		addMembers(g.Maintainers, internal.Maintainer)
		addMembers(g.Owners, internal.Owner)

		l.addGroup(group)
	}

	unhandledGroups := make([]string, 0)
	for _, g := range q.Groups() {
		_, found := l.Group(g)
		if !found {
			unhandledGroups = append(unhandledGroups, g)
		}
	}
	sort.Slice(unhandledGroups, func(i, j int) bool {
		return unhandledGroups[i] < unhandledGroups[j]
	})

	l.unhandledGroups = unhandledGroups

	for projectPath, acls := range c.Projects {
		if !q.ProjectExists(projectPath) {
			errs.Append(fmt.Errorf("project '%s' does not exist", projectPath))
			continue
		}

		project := &LocalProject{
			Fullpath:     projectPath,
			SharedGroups: make(map[string]internal.Level, 0),
			Members:      make(map[string]internal.Level, 0),

			Variables: make(map[string]string, 0),
		}

		for k, envKey := range acls.Variables {
			value, ok := os.LookupEnv(envKey)
			if !ok {
				errs.Append(fmt.Errorf("Project contains secret '%s'='%s' which is not loaded in the environment", k, envKey))
				continue
			}
			project.Variables[k] = value
		}

		addSharedGroups := func(members []string, level internal.Level) {
			for _, member := range members {
				if strings.HasPrefix(member, "share_with:") {
					member = strings.TrimSpace(member[11:])
					if !q.GroupExists(member) {
						errs.Append(fmt.Errorf("can't share project '%s' with non-existing group '%s'", projectPath, member))
						continue
					}
					project.addGroupSharing(member, level)
					continue
				}

				if strings.HasPrefix(member, "query:") {
					queries = append(queries, query{
						query:       strings.TrimSpace(member[6:]),
						level:       level,
						memberAdder: project,
					})
					continue
				}

				if !q.IsUser(member) && !q.IsAdmin(member) {
					errs.Append(fmt.Errorf("User '%s' does not exists for project '%s'", member, project))
					continue
				}

				project.addMember(member, level)
			}
		}

		addSharedGroups(acls.Owners, internal.Owner)
		addSharedGroups(acls.Maintainers, internal.Maintainer)
		addSharedGroups(acls.Developers, internal.Developer)
		addSharedGroups(acls.Reporters, internal.Reporter)
		addSharedGroups(acls.Guests, internal.Guest)

		l.addProject(project)
	}

	for _, query := range queries {
		if err := query.Execute(l, q); err != nil {
			errs.Append(fmt.Errorf("failed to execute query %s: %s", query, err))
		}
	}

Loop:
	for _, localGroup := range l.groups {
		for _, level := range localGroup.Members {
			if level == internal.Owner {
				continue Loop
			}
		}
		errs.Append(fmt.Errorf("no owner in group '%s'", localGroup.Fullpath))
	}

	for _, u := range c.Users.Admins {
		l.admins[u] = 1
	}

	for _, u := range c.Users.Blocked {
		l.blocked[u] = 1
	}

	return l, errs.ErrorOrNil()
}

var queryMatch = regexp.MustCompile("^(.*?) (?:from|in) (.*?)$")

type query struct {
	query       string
	level       internal.Level
	memberAdder memberAdder
}

type memberAdder interface {
	addMember(member string, level internal.Level)
	String() string
}

func (q query) String() string {
	return fmt.Sprintf("'%s' for '%s/%s'", q.query, q.memberAdder, q.level)
}

func (q query) Execute(state localState, querier internal.Querier) error {
	addMembers := func(members []string) error {
		for _, member := range members {
			q.memberAdder.addMember(member, q.level)
		}
		return nil
	}

	switch q.query {
	case "users":
		addMembers(querier.Users())
		break

	case "admins":
		addMembers(querier.Admins())
		break

	default:
		matching := queryMatch.FindAllStringSubmatch(q.query, -1)
		if len(matching) == 0 {
			return fmt.Errorf("Invalid query '%s'", q.query)
		}
		logrus.Debugf("matching query: %#v", matching)

		queriedACL, queriedGroupName := matching[0][1], matching[0][2]
		grp, ok := state.Group(queriedGroupName)
		if !ok {
			return fmt.Errorf("could not find group '%s' to resolve query '%s' in '%s/%s'",
				queriedGroupName, q.query, q.memberAdder, q.level)
		}
		queriedGroup := grp.(*LocalGroup)
		if queriedGroup.HasSubquery() {
			return fmt.Errorf("group '%s' points at '%s/%s' which contains '%s'. Subquerying is not allowed",
				queriedGroupName, q.memberAdder, q.level, q.query)
		}

		filterByLevel := func(members map[string]internal.Level, level internal.Level) []string {
			matched := make([]string, 0)
			for u, l := range members {
				if l == level {
					matched = append(matched, u)
				}
			}
			return matched
		}
		filterByAdminness := func(members map[string]internal.Level, shouldBeAdmin bool) []string {
			matched := make([]string, 0)
			for u := range members {
				switch shouldBeAdmin {
				case true:
					if querier.IsAdmin(u) {
						matched = append(matched, u)
					}
				default:
					if querier.IsUser(u) {
						matched = append(matched, u)
					}
				}
			}
			return matched
		}

		switch strings.Title(queriedACL) {
		case "Guests":
			return addMembers(filterByLevel(queriedGroup.GetMembers(), internal.Guest))

		case "Reporters":
			return addMembers(filterByLevel(queriedGroup.GetMembers(), internal.Reporter))

		case "Developers":
			return addMembers(filterByLevel(queriedGroup.GetMembers(), internal.Developer))

		case "Maintainers":
			return addMembers(filterByLevel(queriedGroup.GetMembers(), internal.Maintainer))

		case "Owners":
			return addMembers(filterByLevel(queriedGroup.GetMembers(), internal.Owner))

		case "Admins":
			return addMembers(filterByAdminness(queriedGroup.GetMembers(), true))

		case "Users":
			return addMembers(filterByAdminness(queriedGroup.GetMembers(), false))

		}
	}
	return nil
}
