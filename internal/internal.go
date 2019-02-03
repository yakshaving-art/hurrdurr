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

	GetVariables() map[string]string
	HasVariable(key string) bool
	VariableEquals(key, value string) bool
}

// Project represents a gitlab project
type Project interface {
	GetFullpath() string
	GetGroupLevel(string) (Level, bool)

	GetSharedGroups() map[string]Level
	GetMembers() map[string]Level

	GetVariables() map[string]string
	HasVariable(key string) bool
	VariableEquals(key, value string) bool
}

// State represents a state which includes groups and memberships
type State interface {
	Groups() []Group
	Group(name string) (Group, bool)
	UnhandledGroups() []string

	Projects() []Project
	Project(string) (Project, bool)

	Admins() []string
	IsAdmin(string) bool

	Blocked() []string
	IsBlocked(string) bool

	CurrentUser() string
}

// Querier represents an object which can be used to query a live instance to validate data
type Querier interface {
	IsUser(string) bool
	IsAdmin(string) bool
	IsBlocked(u string) bool
	GroupExists(string) bool
	ProjectExists(string) bool

	CurrentUser() string
	Users() []string
	GetUserID(string) int
	Groups() []string
	GetGroupID(string) int
	Admins() []string
	Blocked() []string
	Projects() []string
}

// ActionPriority is used to prioritize different actions according to when
// should they be executed
type ActionPriority int

// Priorities
const (
	UnblockUser     = 0
	ManageAdminUser = 1
	ManageGroup     = 2
	ManageProject   = 3
	BlockUser       = 4
)

// Action is an action to execute using the APIClient
type Action interface {
	Execute(APIClient) error
	Priority() ActionPriority
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

	CreateGroupVariable(group, key, value string) error
	UpdateGroupVariable(group, key, value string) error

	CreateProjectVariable(fullpath, key, value string) error
	UpdateProjectVariable(fullpath, key, value string) error

	BlockUser(username string) error
	UnblockUser(username string) error

	SetAdminUser(username string) error
	UnsetAdminUser(username string) error
}

// Config represents the configuration structure supporter by hurrdurr
type Config struct {
	Groups   map[string]Acls `yaml:"groups,omitempty"`
	Projects map[string]Acls `yaml:"projects,omitempty"`
	Users    Users           `yaml:"users,omitempty"`
}

// Acls represents a set of levels and users in each level in a configuration file
type Acls struct {
	Guests      []string          `yaml:"guests,omitempty"`
	Reporters   []string          `yaml:"reporters,omitempty"`
	Developers  []string          `yaml:"developers,omitempty"`
	Maintainers []string          `yaml:"maintainers,omitempty"`
	Owners      []string          `yaml:"owners,omitempty"`
	Variables   map[string]string `yaml:"secret_variables,omitempty"`
}

// Users represents the pair of admins and blocked users
type Users struct {
	Admins  []string `yaml:"admins,omitempty"`
	Blocked []string `yaml:"blocked,omitempty"`
}
