package state_test

import (
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"
	"testing"

	"gitlab.com/yakshaving.art/hurrdurr/internal/api"
	"gitlab.com/yakshaving.art/hurrdurr/internal/state"

	"github.com/stretchr/testify/assert"
)

var querier = querierMock{
	admins: map[string]bool{
		"admin": true,
	},
	users: map[string]bool{
		"user1": true,
		"user2": true,
		"user3": true,
	},
	groups: map[string]bool{
		"root_group":  true,
		"skrrty":      true,
		"other_group": true,
	},
	projects: map[string]bool{
		"root_group/a_project":        true,
		"root_group/myawesomeproject": true,
	},
}

func TestDiffWithoutOneStateFails(t *testing.T) {
	a := assert.New(t)

	c, err := util.LoadConfig("fixtures/plain.yaml")
	a.NoError(err)

	s, err := state.LoadStateFromFile(c, querier)
	a.NoError(err)

	_, err = state.Diff(nil, s, state.DiffArgs{})
	a.EqualError(err, "invalid current state: nil")

	_, err = state.Diff(s, nil, state.DiffArgs{})
	a.EqualError(err, "invalid desired state: nil")
}

func TestDiffingStates(t *testing.T) {

	tt := []struct {
		name           string
		sourceState    string
		desiredState   string
		desiredActions []string
	}{
		{
			"add developers",
			"fixtures/diff-root-with-admin.yaml",
			"fixtures/diff-root-with-2-developers.yaml",
			[]string{
				"add 'user1' to 'root_group' at level 'Developer'",
				"add 'user2' to 'root_group' at level 'Developer'",
			},
		},
		{
			"add skrrty group and change admin",
			"fixtures/diff-root-with-admin.yaml",
			"fixtures/diff-with-skrrty-group.yaml",
			[]string{"add 'user1' to 'root_group' at level 'Owner'",
				"remove 'admin' from 'root_group'",
				"add 'user1' to 'skrrty' at level 'Developer'",
				"add 'user2' to 'skrrty' at level 'Developer'",
				"add 'admin' to 'skrrty' at level 'Owner'",
			},
		},
		{
			"change admins in developers",
			"fixtures/diff-root-with-2-admins.yaml",
			"fixtures/diff-root-with-2-developers.yaml",
			[]string{
				"change 'user1' in 'root_group' at level 'Developer'",
				"add 'user2' to 'root_group' at level 'Developer'",
			},
		},
		{
			"share root with skrrty",
			"fixtures/diff-with-skrrty-group.yaml",
			"fixtures/diff-share-root-with-skrrty-group-as-maintainers.yaml",
			[]string{
				"share project 'root_group/myawesomeproject' with group 'skrrty' at level 'Maintainer'",
				"add 'user1' to 'root_group/myawesomeproject' at level 'Developer'",
			},
		},
		{
			"unshare root with skrrty",
			"fixtures/diff-share-root-with-skrrty-group-as-maintainers.yaml",
			"fixtures/diff-with-skrrty-group.yaml",
			[]string{
				"remove project sharing from 'root_group/myawesomeproject' with group 'skrrty'",
				"remove 'user1' from 'root_group/myawesomeproject'",
			},
		},
		{
			"change root sharing with skrrty to developers",
			"fixtures/diff-share-root-with-skrrty-group-as-maintainers.yaml",
			"fixtures/diff-share-root-with-skrrty-group-as-developers.yaml",
			[]string{
				"remove project sharing from 'root_group/myawesomeproject' with group 'skrrty'",
				"share project 'root_group/myawesomeproject' with group 'skrrty' at level 'Developer'",
				"remove 'user1' from 'root_group/myawesomeproject'",
			},
		},
		{
			"change root sharing with skrrty to maintainers",
			"fixtures/diff-share-root-with-skrrty-group-as-developers.yaml",
			"fixtures/diff-share-root-with-skrrty-group-as-maintainers.yaml",
			[]string{
				"remove project sharing from 'root_group/myawesomeproject' with group 'skrrty'",
				"share project 'root_group/myawesomeproject' with group 'skrrty' at level 'Maintainer'",
				"add 'user1' to 'root_group/myawesomeproject' at level 'Developer'",
			},
		},
		{
			"add project permissions",
			"fixtures/plain.yaml",
			"fixtures/plain-with-project.yaml",
			[]string{
				"share project 'root_group/a_project' with group 'other_group' at level 'Developer'",
				"add 'admin' to 'root_group/a_project' at level 'Owner'",
				"add 'user2' to 'root_group/a_project' at level 'Maintainer'",
			},
		},
		{
			"change project permissions",
			"fixtures/plain-with-project.yaml",
			"fixtures/plain-with-other-levels-project.yaml",
			[]string{
				"add 'user1' to 'root_group/a_project' at level 'Developer'",
				"change 'user2' in 'root_group/a_project' to level 'Developer'",
				"add 'user3' to 'root_group/a_project' at level 'Reporter'",
				"remove project sharing from 'root_group/a_project' with group 'other_group'",
			},
		},
		{
			"change project permissions the other way",
			"fixtures/plain-with-other-levels-project.yaml",
			"fixtures/plain-with-project.yaml",
			[]string{
				"remove 'user1' from 'root_group/a_project'",
				"remove 'user3' from 'root_group/a_project'",
				"change 'user2' in 'root_group/a_project' to level 'Maintainer'",
				"share project 'root_group/a_project' with group 'other_group' at level 'Developer'",
			},
		},
		{
			"plain project permissions without changes",
			"fixtures/plain-with-project.yaml",
			"fixtures/plain-with-project.yaml",
			[]string{},
		},
		{
			"blocking a user works",
			"fixtures/plain-with-admins.yaml",
			"fixtures/plain-with-blocked-user.yaml",
			[]string{
				"unset 'user3' as admin",
				"block 'user3'",
			},
		},
		{
			"unblocking a user works",
			"fixtures/plain-with-blocked-user.yaml",
			"fixtures/plain-with-admins.yaml",
			[]string{
				"set 'user3' as admin",
				"unblock 'user3'",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			sourceConfig, err := util.LoadConfig(tc.sourceState)
			a.NoError(err, "source state")

			sourceState, err := state.LoadStateFromFile(sourceConfig, querier)
			a.NoError(err, "source state")

			desiredConfig, err := util.LoadConfig(tc.desiredState)
			a.NoError(err, "desired state")

			desiredState, err := state.LoadStateFromFile(desiredConfig, querier)
			a.NoError(err, "desired state")

			actions, err := state.Diff(sourceState, desiredState, state.DiffArgs{
				DiffGroups:   true,
				DiffProjects: true,
				DiffUsers:    true,
			})
			a.NoError(err, "diff")
			a.NotNil(actions, "actions")

			executedActions := make([]string, 0)
			c := api.DryRunAPIClient{
				Append: func(action string) {
					executedActions = append(executedActions, action)
				},
			}

			for _, action := range actions {
				a.NoError(action.Execute(c))
			}

			// a.Equal(len(tc.actions), len(executedActions), "actions length is not as expected")
			// a.Equal(tc.desiredActions, executedActions, "actions are not as expected")

			for _, action := range tc.desiredActions {
				a.Contains(executedActions, action, "more desired actions than executed")
			}
			for _, action := range executedActions {
				a.Contains(tc.desiredActions, action, "more executed actions than desired")
			}

		})
	}
}
