package state_test

import (
	"testing"

	"gitlab.com/yakshaving.art/hurrdurr/internal/api"
	"gitlab.com/yakshaving.art/hurrdurr/internal/state"

	"github.com/stretchr/testify/assert"
)

func TestDiffingStates(t *testing.T) {
	querier := querierMock{
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
	}

	tt := []struct {
		name         string
		sourceState  string
		desiredState string
		actions      []string
	}{
		{
			"add developers",
			"fixtures/diff-root-with-admin.yaml",
			"fixtures/diff-root-with-2-developers.yaml",
			[]string{"add 'user1' to 'root_group' at level '30'",
				"add 'user2' to 'root_group' at level '30'"},
		},
		{
			"add skrrty group and change admin",
			"fixtures/diff-root-with-admin.yaml",
			"fixtures/diff-with-skrrty-group.yaml",
			[]string{"add 'user1' to 'root_group' at level '50'",
				"remove 'admin' from 'root_group'",
				"add 'user1' to 'skrrty' at level '30'",
				"add 'user2' to 'skrrty' at level '30'",
				"add 'admin' to 'skrrty' at level '50'",
			},
		},
		{
			"change admins for developers",
			"fixtures/diff-root-with-2-admins.yaml",
			"fixtures/diff-root-with-2-developers.yaml",
			[]string{
				"change 'user1' to 'root_group' at level '30'",
				"add 'user2' to 'root_group' at level '30'",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			sourceState, err := state.LoadStateFromFile(tc.sourceState, querier)
			a.NoError(err, "source state")

			desiredState, err := state.LoadStateFromFile(tc.desiredState, querier)
			a.NoError(err, "desired state")

			actions, err := state.Diff(sourceState, desiredState)
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

			a.Equal(len(tc.actions), len(executedActions), "actions length is not as expected")

			for _, action := range tc.actions {
				a.Contains(executedActions, action, "action missing")
			}
			for _, action := range executedActions {
				a.Contains(tc.actions, action, "action missing")
			}

		})
	}
}
