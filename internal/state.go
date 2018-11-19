package internal

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"

	yaml "github.com/ghodss/yaml"
)

// Level represents the access level granted to a user in a group
type Level int

// Levels definitions
const (
	Guest      = 10
	Reporter   = 20
	Developer  = 30
	Maintainer = 40
	Owner      = 50
)

func (l Level) String() string {
	levels := [...]string{
		"Guest",
		"Reporter",
		"Developer",
		"Maintainer",
		"Owner",
	}
	if l < Guest || l > Owner {
		return "Unknown"
	}
	return levels[(l-Guest)/10]
}

// Group represents a group with a fullpath and it's members
type Group struct {
	Fullpath    string
	Members     []Membership
	HasSubquery bool
}

// Membership represents the membership of a single user to a given group
type Membership struct {
	Username string
	Level    Level
}

// State represents a state which includes groups and memberships
type State interface {
	Groups() []Group
	Group(name string) (Group, bool)
}

// Querier represents an object which can be used to query a live instance to validate data
type Querier interface {
	IsUser(string) bool
	IsAdmin(string) bool
	GroupExists(string) bool

	Users() []string
	Admins() []string
}

// LoadStateFromFile loads the desired state from a file
func LoadStateFromFile(filename string, q Querier) (State, error) {
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
	groups map[string]*Group
}

func (s localState) Groups() []Group {
	groups := make([]Group, 0)
	for _, g := range s.groups {
		groups = append(groups, *g)
	}
	return groups
}

func (s localState) Group(name string) (Group, bool) {
	g, ok := s.groups[name]
	return *g, ok
}

type acls struct {
	Guests      []string `yaml:"guests,omitempty"`
	Reporters   []string `yaml:"reporters,omitempty"`
	Developers  []string `yaml:"developers,omitempty"`
	Maintainers []string `yaml:"maintainers,omitempty"`
	Owners      []string `yaml:"owners,omitempty"`
}

type state struct {
	Groups map[string]acls `yaml:"groups"`
}

func (s state) toLocalState(q Querier) (localState, error) {
	l := localState{
		groups: make(map[string]*Group, 0),
	}

	errs := errors.New() // This object aggregates all the errors to dump them all at the end
	queries := make([]query, 0)

	for fullpath, g := range s.Groups {
		if !q.GroupExists(fullpath) {
			errs.Append(fmt.Errorf("Group %s does not exist", fullpath))
			continue
		}

		group := Group{
			Fullpath: fullpath,
			Members:  make([]Membership, 0),
		}

		addMembers := func(members []string, level Level) {
			for _, member := range members {
				if strings.HasPrefix(member, "query:") {
					queries = append(queries, query{
						query:    strings.TrimSpace(member[6:]),
						level:    level,
						fullpath: fullpath,
					})
					group.HasSubquery = true
					continue
				}
				if !q.IsUser(member) && !q.IsAdmin(member) {
					errs.Append(fmt.Errorf("User %s does not exists for group %s", member, fullpath))
					continue
				}

				group.Members = append(group.Members, Membership{
					Username: member,
					Level:    level,
				})
			}
		}

		addMembers(g.Guests, Guest)
		addMembers(g.Reporters, Reporter)
		addMembers(g.Developers, Developer)
		addMembers(g.Maintainers, Maintainer)
		addMembers(g.Owners, Owner)

		l.groups[fullpath] = &group
	}

	for _, query := range queries {
		if err := query.Execute(l, q); err != nil {
			errs.Append(fmt.Errorf("failed to execute query %s: %s", query, err))
		}
	}

	return l, errs.ErrorOrNil()
}

var queryMatch = regexp.MustCompile("^(.*?) (?:from|in) (.*?)$")

type query struct {
	query    string
	level    Level
	fullpath string
}

func (q query) String() string {
	return fmt.Sprintf("'%s' for %s/%s", q.query, q.fullpath, q.level)
}

func (q query) Execute(state localState, querier Querier) error {
	group, ok := state.groups[q.fullpath]
	if !ok {
		return fmt.Errorf("could not find group in list %s", q.fullpath)
	}

	addMembers := func(members []string) error {
		for _, member := range members {
			group.Members = append(group.Members, Membership{
				Username: member,
				Level:    q.level,
			})
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

		queriedACL, queriedGroupName := matching[0][1], matching[0][2]
		queriedGroup, ok := state.Group(queriedGroupName)
		if !ok {
			return fmt.Errorf("could not find group %s to resolve query '%s' from %s/%s",
				queriedGroupName, q.query, q.fullpath, q.level)
		}
		if queriedGroup.HasSubquery {
			return fmt.Errorf("group %s pointed at from %s/%s contains a query '%s'. This is not allowed",
				queriedGroupName, q.fullpath, q.level, q.query)
		}

		filterByLevel := func(members []Membership, level Level) []string {
			matched := make([]string, 0)
			for _, m := range members {
				if m.Level == level {
					matched = append(matched, m.Username)
				}
			}
			return matched
		}
		filterByAdminness := func(members []Membership, shouldBeAdmin bool) []string {
			matched := make([]string, 0)
			for _, m := range members {
				switch shouldBeAdmin {
				case true:
					if querier.IsAdmin(m.Username) {
						matched = append(matched, m.Username)
					}
				default:
					if querier.IsUser(m.Username) {
						matched = append(matched, m.Username)
					}
				}
			}
			return matched
		}

		switch strings.Title(queriedACL) {
		case "Guests":
			return addMembers(filterByLevel(queriedGroup.Members, Guest))

		case "Reporters":
			return addMembers(filterByLevel(queriedGroup.Members, Reporter))

		case "Developers":
			return addMembers(filterByLevel(queriedGroup.Members, Developer))

		case "Maintainers":
			return addMembers(filterByLevel(queriedGroup.Members, Maintainer))

		case "Owners":
			return addMembers(filterByLevel(queriedGroup.Members, Owner))

		case "Admins":
			return addMembers(filterByAdminness(queriedGroup.Members, true))

		case "Users":
			return addMembers(filterByAdminness(queriedGroup.Members, false))

		}
	}
	return nil
}
