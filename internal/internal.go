package internal

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

// Group represents a gitlab group
type Group interface {
	GetFullpath() string
	GetMembers() map[string]Level
}

// Project represents a gitlab project
type Project interface {
	GetFullpath() string
	GetSharedGroups() map[string]Level
	GetGroupLevel(string) (Level, bool)
}

// State represents a state which includes groups and memberships
type State interface {
	Groups() []Group
	Group(name string) (Group, bool)
	UnhandledGroups() []string

	Projects() []Project
	Project(string) (Project, bool)
}

// Querier represents an object which can be used to query a live instance to validate data
type Querier interface {
	IsUser(string) bool
	IsAdmin(string) bool
	GroupExists(string) bool
	ProjectExists(string) bool

	Users() []string
	Groups() []string
	Admins() []string
}

// Action is an action to execute using the APIClient
type Action interface {
	Execute(APIClient) error
}

// APIClient is the tool used to reach the remote instance and perform actions on it
type APIClient interface {
	AddGroupMembership(username, group string, level Level) error
	ChangeGroupMembership(username, group string, level Level) error
	RemoveGroupMembership(username, group string) error

	AddProjectSharing(project, group string, level Level) error
	RemoveProjectSharing(project, group string) error

	AddProjectMembership(username, project string, level Level) error
	ChangeProjectMembership(username, project string, level Level) error
	RemoveProjectMembership(username, project string) error
}
