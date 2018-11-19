package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	hurrdurr "gitlab.com/yakshaving.art/hurrdurr/internal"
)

func TestLoadingState(t *testing.T) {
	querier := querierMock{
		admins: map[string]bool{
			"admin": true,
		},
		users: map[string]bool{
			"user1": true,
		},
		groups: map[string]bool{
			"root_group": true,
			"skrrty":     true,
		},
	}
	tt := []struct {
		name          string
		stateFile     string
		expectedError string
		expected      []hurrdurr.Group
	}{
		{
			"non existing file",
			"",
			"failed to load state file : open : no such file or directory",
			nil,
		},
		{
			"non existing user and group",
			"fixtures/non_existing.yaml",
			"failed to build local state from file fixtures/non_existing.yaml: 2 errors: Group non_existing_group does not exist; User non_exiting does not exists for group root_group",
			nil,
		},
		{
			"plain state",
			"fixtures/plain.yaml",
			"",
			[]hurrdurr.Group{
				{
					Fullpath: "root_group",
					Members: []hurrdurr.Membership{
						{
							Username: "user1",
							Level:    hurrdurr.Developer,
						},
						{
							Username: "admin",
							Level:    hurrdurr.Owner,
						},
					},
				},
			},
		},
		{
			"valid queries",
			"fixtures/valid-queries.yaml",
			"",
			[]hurrdurr.Group{
				{
					Fullpath: "root_group",
					Members: []hurrdurr.Membership{
						{
							Username: "admin",
							Level:    hurrdurr.Owner,
						},
					},
				},
				{
					Fullpath: "skrrty",
					Members: []hurrdurr.Membership{
						{
							Username: "user1",
							Level:    hurrdurr.Developer,
						},
						{
							Username: "admin",
							Level:    hurrdurr.Owner,
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			s, err := hurrdurr.LoadStateFromFile(tc.stateFile, querier)
			if tc.expectedError != "" {
				a.EqualErrorf(err, tc.expectedError, "Wrong error, expected %s, got %s", tc.expectedError, err)
				return
			}

			if err != nil {
				t.Fatalf("failed to read fixture file %s: %s", tc.stateFile, err)
			}
			a.EqualValuesf(tc.expected, s.Groups(), "Wrong state, groups are not as expected")
		})
	}
}

type querierMock struct {
	admins map[string]bool
	users  map[string]bool
	groups map[string]bool
}

func (q querierMock) IsUser(u string) bool {
	_, ok := q.users[u]
	return ok
}

func (q querierMock) IsAdmin(u string) bool {
	_, ok := q.admins[u]
	return ok
}
func (q querierMock) GroupExists(g string) bool {
	_, ok := q.groups[g]
	return ok
}

func (q querierMock) Users() []string {
	users := make([]string, 0)
	for u := range q.users {
		users = append(users, u)
	}
	return users
}

func (q querierMock) Admins() []string {
	admins := make([]string, 0)
	for a := range q.admins {
		admins = append(admins, a)
	}
	return admins
}
