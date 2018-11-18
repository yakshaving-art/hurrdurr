package internal

import (
	"fmt"
	"io/ioutil"

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
	if l < Guest || l > Reporter {
		return "Unknown"
	}
	return levels[l/10]
}

// Group represents a group with a name, namespace and it's members
type Group struct {
	Namespace string
	Name      string
	Members   []Membership
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

// LoadStateFromFile loads the desired state from a file
func LoadStateFromFile(filename string) (State, error) {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load state file %s: %s", filename, err)
	}

	s := state{}
	if err := yaml.Unmarshal(content, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state file %s: %s", filename, err)
	}

	l, err := s.ToLocalState()
	if err != nil {
		return nil, fmt.Errorf("failed to build local state from file %s: %s", filename, err)
	}

	return l, nil
}

type localState struct {
	groups map[string]Group
}

func (s localState) Groups() []Group {
	groups := make([]Group, 0)
	for _, g := range s.groups {
		groups = append(groups, g)
	}
	return groups
}

func (s localState) Group(name string) (Group, bool) {
	g, ok := s.groups[name]
	return g, ok
}

type acls struct {
	Guests      []string `yaml:"guests,omitempty"`
	Reporters   []string `yaml:"reporters,omitempty"`
	Developers  []string `yaml:"developers,omitempty"`
	Maintainers []string `yaml:"maintainers,omitempty"`
	Owners      []string `yaml:"owners,omitempty"`
}

type state struct {
	Groups map[string]acls
}

func (s state) ToLocalState() (localState, error) {
	l := localState{
		groups: make(map[string]Group, 0),
	}

	return l, nil
}
