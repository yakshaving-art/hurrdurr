package internal

import (
	"fmt"
	"io/ioutil"
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
	Group(name string) Group
}

// LoadStateFromFile loads the desired state from a file
func LoadStateFromFile(filename string) (State, error) {
	_, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to load state file %s: %s", filename, err)
	}
	return nil, nil
}
