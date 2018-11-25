package state

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"

	"github.com/go-yaml/yaml"
	"github.com/sirupsen/logrus"
)

// LocalGroup represents a group with a fullpath and it's members that is loaded from a yaml file
type LocalGroup struct {
	Fullpath string
	Members  map[string]internal.Level
	Subquery bool
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

func (g *LocalGroup) addMember(username string, level internal.Level) {
	l, ok := g.Members[username]
	if ok && l > level {
		return
	}
	g.Members[username] = level
}

func (g *LocalGroup) setHasSubquery(b bool) {
	g.Subquery = b
}

// LoadStateFromFile loads the desired state from a file
func LoadStateFromFile(filename string, q internal.Querier) (internal.State, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load state file %s: %s", filename, err)
	}

	s := state{}
	if err := yaml.Unmarshal(content, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state file %s: %s", filename, err)
	}

	l, err := s.toLocalState(q)
	if err != nil {
		return nil, fmt.Errorf("failed to build local state from file %s: %s", filename, err)
	}

	return l, nil
}

type localState struct {
	groups map[string]*LocalGroup
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

type acls struct {
	Guests      []string `yaml:"guests,omitempty"`
	Reporters   []string `yaml:"reporters,omitempty"`
	Developers  []string `yaml:"developers,omitempty"`
	Maintainers []string `yaml:"maintainers,omitempty"`

	Owners []string `yaml:"owners,omitempty"`
}

type state struct {
	Groups map[string]acls `yaml:"groups"`
}

func (s state) toLocalState(q internal.Querier) (localState, error) {
	l := localState{
		groups: make(map[string]*LocalGroup, 0),
	}

	errs := errors.New() // This object aggregates all the errors to dump them all at the end
	queries := make([]query, 0)

	for fullpath, g := range s.Groups {
		if !q.GroupExists(fullpath) {
			errs.Append(fmt.Errorf("Group '%s' does not exist", fullpath))
			continue
		}

		group := &LocalGroup{
			Fullpath: fullpath,
			Members:  make(map[string]internal.Level, 0),
		}

		addMembers := func(members []string, level internal.Level) {
			for _, member := range members {
				if strings.HasPrefix(member, "query:") {
					queries = append(queries, query{
						query:    strings.TrimSpace(member[6:]),
						level:    level,
						fullpath: fullpath,
					})
					group.setHasSubquery(true)
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

	return l, errs.ErrorOrNil()
}

var queryMatch = regexp.MustCompile("^(.*?) (?:from|in) (.*?)$")

type query struct {
	query    string
	level    internal.Level
	fullpath string
}

func (q query) String() string {
	return fmt.Sprintf("'%s' for '%s/%s'", q.query, q.fullpath, q.level)
}

func (q query) Execute(state localState, querier internal.Querier) error {
	group, ok := state.groups[q.fullpath]
	if !ok {
		return fmt.Errorf("could not find group in list %s", q.fullpath)
	}

	addMembers := func(members []string) error {
		for _, member := range members {
			group.addMember(member, q.level)
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
				queriedGroupName, q.query, q.fullpath, q.level)
		}
		queriedGroup := grp.(*LocalGroup)
		if queriedGroup.HasSubquery() {
			return fmt.Errorf("group '%s' points at '%s/%s' which contains '%s'. Subquerying is not allowed",
				queriedGroupName, q.fullpath, q.level, q.query)
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
