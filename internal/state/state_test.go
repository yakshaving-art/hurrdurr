package state_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	hurrdurr "gitlab.com/yakshaving.art/hurrdurr/internal/state"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"
)

func TestLoadingState(t *testing.T) {
	querier := querierMock{
		currentUser: "admin",
		admins: map[string]bool{
			"admin": true,
		},
		users: map[string]bool{
			"user1": true,
			"user2": true,
			"user3": true,
			"user4": true,
		},
		groups: map[string]bool{
			"root_group":        true,
			"skrrty":            true,
			"other_group":       true,
			"simple_group":      true,
			"yet_another_group": true,
		},
		projects: map[string]bool{
			"root_group/a_project": true,
		},
		blocked: map[string]bool{
			"bad_actor_1": true,
		},
	}
	tt := []struct {
		name                    string
		stateFile               string
		expectedError           string
		expected                []hurrdurr.LocalGroup
		expectedUnhandledGroups []string
		expectedProjects        []hurrdurr.LocalProject
	}{
		{
			"non existing file",
			"",
			"failed to load state file : open : no such file or directory",
			nil,
			nil,
			nil,
		},
		// {
		// 	"group without owner fails",
		// 	"fixtures/group-without-owner.yaml",
		// 	"failed to build local state: 1 error: no owner in group 'root_group'",
		// 	nil,
		// 	nil,
		// 	nil,
		// },
		{
			"group with blocked user fails",
			"fixtures/bad-actor.yaml",
			"failed to build local state: 1 error: User 'bad_actor_1' is blocked, it should not be included in group 'root_group'",
			nil,
			nil,
			nil,
		},
		{
			"invalid yaml fails",
			"fixtures/invalid.yaml",
			"failed to unmarshal state file fixtures/invalid.yaml: yaml: unmarshal errors:\n" +
				"  line 6: field dvlprs not found in type internal.Acls",
			nil,
			nil,
			nil,
		},
		// {
		// 	"query for owner returns nothing",
		// 	"fixtures/no-owner-in-query.yaml",
		// 	"failed to build local state: 1 error: no owner in group 'skrrty'",
		// 	nil,
		// 	nil,
		// 	nil,
		// },
		{
			"non existing user and group",
			"fixtures/non_existing.yaml",
			"failed to build local state: " +
				"2 errors: Group 'non_existing_group' does not exist; " +
				"User 'non_existing' does not exist for group 'root_group'",
			nil,
			nil,
			nil,
		},
		{
			"invalid because of subqueries",
			"fixtures/invalid-with-subqueries.yaml",
			"failed to build local state: " +
				"1 error: failed to execute query 'owners from root_group' for 'skrrty/Guest': " +
				"group 'root_group' points at 'skrrty/Guest' which contains 'owners from root_group'. " +
				"Subquerying is not allowed",
			[]hurrdurr.LocalGroup{},
			nil,
			nil,
		},
		{
			"invalid because of non existing group in query",
			"fixtures/invalid-subquery.yaml",
			"failed to build local state: " +
				"2 errors: failed to execute query 'guests from non_existing_group' " +
				"for 'root_group/Guest': could not find group 'non_existing_group' " +
				"to resolve query 'guests from non_existing_group' in 'root_group/Guest'; " +
				"failed to execute query 'whatever from root_group' for 'root_group/Reporter': " +
				"group 'root_group' points at 'root_group/Reporter' which contains " +
				"'whatever from root_group'. Subquerying is not allowed",
			[]hurrdurr.LocalGroup{},
			nil,
			nil,
		},
		{
			"invalid because of blocked user being assigned",
			"fixtures/invalid-with-blocked-user.yaml",
			"failed to build local state: 1 error: User 'bad_actor_1' is blocked, it should not be included in group 'root_group'",
			[]hurrdurr.LocalGroup{},
			nil,
			nil,
		},
		{
			"plain state",
			"fixtures/plain.yaml",
			"",
			[]hurrdurr.LocalGroup{
				{
					Fullpath: "other_group",
					Members: map[string]internal.Level{
						"user2": internal.Owner,
					},
					Variables: map[string]string{},
				},
				{
					Fullpath: "root_group",
					Members: map[string]internal.Level{
						"admin": internal.Owner,
						"user1": internal.Developer,
					},
					Variables: map[string]string{},
				},
			},
			[]string{"simple_group", "skrrty", "yet_another_group"},
			[]hurrdurr.LocalProject{},
		},
		{
			"plain state with project",
			"fixtures/plain-with-project.yaml",
			"",
			[]hurrdurr.LocalGroup{
				{
					Fullpath: "other_group",
					Members: map[string]internal.Level{
						"user2": internal.Owner,
					},
					Variables: map[string]string{},
				},
				{
					Fullpath: "root_group",
					Members: map[string]internal.Level{
						"admin": internal.Owner,
						"user1": internal.Developer,
					},
					Variables: map[string]string{},
				},
			},
			[]string{"simple_group", "skrrty", "yet_another_group"},
			[]hurrdurr.LocalProject{
				{
					Fullpath: "root_group/a_project",
					SharedGroups: map[string]internal.Level{
						"other_group": internal.Developer,
					},
					Members: map[string]internal.Level{
						"admin": internal.Maintainer,
						"user2": internal.Developer,
					},
					Variables: map[string]string{},
				},
			},
		},
		{
			"valid queries",
			"fixtures/valid-queries.yaml",
			"",
			[]hurrdurr.LocalGroup{
				{
					Fullpath: "other_group",
					Subquery: true,
					Members: map[string]internal.Level{
						"admin": internal.Owner,
						"user1": internal.Developer,
						"user2": internal.Developer,
						"user3": internal.Developer,
						"user4": internal.Developer,
					},
					Variables: map[string]string{},
				},
				{
					Fullpath: "root_group",
					Members: map[string]internal.Level{
						"admin": internal.Owner,
					},
					Variables: map[string]string{},
				},
				{
					Fullpath: "simple_group",
					Members: map[string]internal.Level{
						"admin": internal.Owner,
						"user1": internal.Maintainer,
						"user2": internal.Developer,
						"user3": internal.Reporter,
						"user4": internal.Guest,
					},
					Variables: map[string]string{},
				},
				{
					Fullpath: "skrrty",
					Subquery: true,
					Members: map[string]internal.Level{
						"admin": internal.Owner,
						"user1": internal.Guest,
						"user2": internal.Guest,
						"user3": internal.Guest,
						"user4": internal.Guest,
					},
					Variables: map[string]string{},
				},
				{
					Fullpath: "yet_another_group",
					Subquery: true,
					Members: map[string]internal.Level{
						"admin": internal.Owner,
						"user1": internal.Maintainer,
						"user2": internal.Developer,
						"user3": internal.Reporter,
						"user4": internal.Guest,
					},
					Variables: map[string]string{},
				},
			},
			[]string{},
			[]hurrdurr.LocalProject{},
		},
		{
			"multi level assignment",
			"fixtures/multi-level-assignment.yaml",
			"",
			[]hurrdurr.LocalGroup{
				{
					Fullpath: "root_group",
					Subquery: true,
					Members: map[string]internal.Level{
						"admin": internal.Owner,
					},
					Variables: map[string]string{},
				},
			},
			[]string{"other_group", "simple_group", "skrrty", "yet_another_group"},
			[]hurrdurr.LocalProject{},
		},
	}

	loadState := func(filename string, querier internal.Querier) (internal.State, error) {
		c, err := util.LoadConfig(filename, false)
		if err != nil {
			return nil, err
		}
		return hurrdurr.LoadStateFromFile(c, querier)

	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			s, err := loadState(tc.stateFile, querier)
			if tc.expectedError != "" {
				a.EqualErrorf(err, tc.expectedError, "Wrong error, expected '%s', got '%s'", tc.expectedError, err)
				return
			}

			if err != nil {
				t.Fatalf("failed to read fixture file %s: %s", tc.stateFile, err)
			}

			actualGroups := make([]hurrdurr.LocalGroup, 0)
			for _, g := range s.Groups() {
				ag := g.(hurrdurr.LocalGroup)
				actualGroups = append(actualGroups, ag)
			}

			sort.Slice(actualGroups, func(i, j int) bool {
				if actualGroups[i].GetFullpath() < actualGroups[j].GetFullpath() {
					return true
				}
				return false
			})
			a.EqualValuesf(tc.expected, actualGroups, "Wrong state, groups are not as expected")

			actualProjects := make([]hurrdurr.LocalProject, 0)
			for _, p := range s.Projects() {
				pj := p.(hurrdurr.LocalProject)
				actualProjects = append(actualProjects, pj)
			}
			a.EqualValues(tc.expectedProjects, actualProjects, "Wrong state, projects are not as expected")
		})
	}
}

type querierMock struct {
	currentUser string
	admins      map[string]bool
	users       map[string]bool
	blocked     map[string]bool
	groups      map[string]bool
	projects    map[string]bool
}

func (q querierMock) CurrentUser() string {
	return q.currentUser
}

func (q querierMock) IsUser(u string) bool {
	_, ok := q.users[u]
	return ok
}

func (q querierMock) IsAdmin(u string) bool {
	_, ok := q.admins[u]
	return ok
}

func (q querierMock) IsBlocked(u string) bool {
	_, ok := q.blocked[u]
	return ok
}

func (q querierMock) GroupExists(g string) bool {
	_, ok := q.groups[g]
	return ok
}
func (q querierMock) Groups() []string {
	groups := make([]string, 0)
	for g := range q.groups {
		groups = append(groups, g)
	}
	return groups
}

func (q querierMock) ProjectExists(p string) bool {
	_, ok := q.projects[p]
	return ok
}

func (q querierMock) Users() []string {
	return q.toStringSlice(q.users)
}

func (q querierMock) Admins() []string {
	return q.toStringSlice(q.admins)
}

func (q querierMock) Blocked() []string {
	return q.toStringSlice(q.blocked)
}

func (q querierMock) Projects() []string {
	return q.toStringSlice(q.projects)
}

func (q querierMock) toStringSlice(m map[string]bool) []string {
	s := make([]string, 0)
	for a := range m {
		s = append(s, a)
	}
	return s
}

func (querierMock) GetGroupID(_ string) int {
	return -1
}

func (querierMock) GetUserID(_ string) int {
	return -1
}
