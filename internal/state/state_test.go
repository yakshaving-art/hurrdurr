package state_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	hurrdurr "gitlab.com/yakshaving.art/hurrdurr/internal/state"
)

func TestLoadingState(t *testing.T) {
	querier := querierMock{
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
	}
	tt := []struct {
		name          string
		stateFile     string
		expectedError string
		expected      []hurrdurr.LocalGroup
	}{
		{
			"non existing file",
			"",
			"failed to load state file : open : no such file or directory",
			nil,
		},
		{
			"group without owner fails",
			"fixtures/group-without-owner.yaml",
			"failed to build local state from file fixtures/group-without-owner.yaml: 1 error: no owner in group 'root_group'",
			nil,
		},
		{
			"query for owner returns nothing",
			"fixtures/no-owner-in-query.yaml",
			"failed to build local state from file fixtures/no-owner-in-query.yaml: 1 error: no owner in group 'skrrty'",
			nil,
		},
		{
			"non existing user and group",
			"fixtures/non_existing.yaml",
			"failed to build local state from file fixtures/non_existing.yaml: " +
				"3 errors: Group 'non_existing_group' does not exist; " +
				"User 'non_exiting' does not exists for group 'root_group'; " +
				"no owner in group 'root_group'",
			nil,
		},
		{
			"invalid because of subqueries",
			"fixtures/invalid-with-subqueries.yaml",
			"failed to build local state from file fixtures/invalid-with-subqueries.yaml: " +
				"1 error: failed to execute query 'owners from root_group' for 'skrrty/Guest': " +
				"group 'root_group' points at 'skrrty/Guest' which contains 'owners from root_group'. " +
				"Subquerying is not allowed",
			[]hurrdurr.LocalGroup{},
		},
		{
			"invalid because of non existing group in query",
			"fixtures/invalid-subquery.yaml",
			"failed to build local state from file fixtures/invalid-subquery.yaml: " +
				"2 errors: failed to execute query 'guests from non_existing_group' " +
				"for 'root_group/Guest': could not find group 'non_existing_group' " +
				"to resolve query 'guests from non_existing_group' in 'root_group/Guest'; " +
				"failed to execute query 'whatever from root_group' for 'root_group/Reporter': " +
				"group 'root_group' points at 'root_group/Reporter' which contains " +
				"'whatever from root_group'. Subquerying is not allowed",
			[]hurrdurr.LocalGroup{},
		},
		{
			"plain state",
			"fixtures/plain.yaml",
			"",
			[]hurrdurr.LocalGroup{
				{
					Fullpath: "root_group",
					Members: map[string]internal.Level{
						"admin": internal.Owner,
						"user1": internal.Developer,
					},
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
				},
				{
					Fullpath: "root_group",
					Members: map[string]internal.Level{
						"admin": internal.Owner,
					},
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
				},
			},
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

			actual := make([]hurrdurr.LocalGroup, 0)
			for _, g := range s.Groups() {
				ag := g.(hurrdurr.LocalGroup)
				actual = append(actual, ag)
			}

			sort.Slice(actual, func(i, j int) bool {
				if actual[i].GetFullpath() < actual[j].GetFullpath() {
					return true
				}
				return false
			})
			a.EqualValuesf(tc.expected, actual, "Wrong state, groups are not as expected")
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