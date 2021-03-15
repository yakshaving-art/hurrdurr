package state_test

import (
	"os"
	"testing"

	"gitlab.com/yakshaving.art/hurrdurr/internal/api"
	"gitlab.com/yakshaving.art/hurrdurr/internal/state"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

var querier = querierMock{
	currentUser: "admin",
	admins: map[string]bool{
		"admin": true,
	},
	users: map[string]bool{
		"user1": true,
		"user2": true,
		"user3": true,
	},
	groups: map[string]bool{
		"root_group":           true,
		"skrrty":               true,
		"other_group":          true,
		"root_group/subgroup1": true,
		"root_group/subgroup2": true,
	},
	projects: map[string]bool{
		"root_group/a_project":        true,
		"root_group/myawesomeproject": true,
	},
}

func TestDiffWithoutOneStateFails(t *testing.T) {
	a := assert.New(t)

	c, err := util.LoadConfig("fixtures/plain.yaml", false)
	a.NoError(err)

	s, err := state.LoadStateFromFile(c, querier)
	a.NoError(err)

	_, err = state.Diff(nil, s, state.DiffArgs{})
	a.EqualError(err, "invalid current state: nil")

	_, err = state.Diff(s, nil, state.DiffArgs{})
	a.EqualError(err, "invalid desired state: nil")
}

func TestDiffRemovingCurrentAdminUserFails(t *testing.T) {
	a := assert.New(t)

	sourceConfig, err := util.LoadConfig("fixtures/plain-with-admins.yaml", false)
	a.NoError(err)

	sourceState, err := state.LoadStateFromFile(sourceConfig, querier)
	a.NoError(err)

	desiredConfig, err := util.LoadConfig("fixtures/plain-without-current-user.yaml", false)
	a.NoError(err)

	desiredState, err := state.LoadStateFromFile(desiredConfig, querier)
	a.NoError(err)

	_, err = state.Diff(sourceState, desiredState, state.DiffArgs{
		DiffUsers: true,
	})
	a.EqualError(err, "2 errors: "+
		"can't block current user 'admin' as it would be shooting myself in the foot; "+
		"can't unset current user 'admin' as admin as it would be shooting myself in the foot")
}

func TestDiffingStates(t *testing.T) {

	tt := []struct {
		name           string
		sourceState    string
		desiredState   string
		desiredActions []string
		inOrder        bool
	}{
		{
			"add developers",
			"fixtures/diff-root-with-admin.yaml",
			"fixtures/diff-root-with-2-developers.yaml",
			[]string{
				"add 'user1' to 'root_group' at level 'Developer'",
				"add 'user2' to 'root_group' at level 'Developer'",
			},
			false,
		},
		{
			"add skrrty group and change admin",
			"fixtures/diff-root-with-admin.yaml",
			"fixtures/diff-with-skrrty-group.yaml",
			[]string{
				"add 'user1' to 'root_group' at level 'Owner'",
				"remove 'admin' from 'root_group'",
				"add 'user1' to 'skrrty' at level 'Developer'",
				"add 'user2' to 'skrrty' at level 'Developer'",
				"add 'admin' to 'skrrty' at level 'Owner'",
			},
			false,
		},
		{
			"change admins in developers",
			"fixtures/diff-root-with-2-admins.yaml",
			"fixtures/diff-root-with-2-developers.yaml",
			[]string{
				"change 'user1' in 'root_group' at level 'Developer'",
				"add 'user2' to 'root_group' at level 'Developer'",
			},
			false,
		},
		{
			"share root with skrrty",
			"fixtures/diff-with-skrrty-group.yaml",
			"fixtures/diff-share-root-with-skrrty-group-as-maintainers.yaml",
			[]string{
				"share project 'root_group/myawesomeproject' with group 'skrrty' at level 'Maintainer'",
				"add 'user1' to 'root_group/myawesomeproject' at level 'Developer'",
			},
			false,
		},
		{
			"unshare root with skrrty",
			"fixtures/diff-share-root-with-skrrty-group-as-maintainers.yaml",
			"fixtures/diff-with-skrrty-group.yaml",
			[]string{
				"remove project sharing from 'root_group/myawesomeproject' with group 'skrrty'",
				"remove 'user1' from 'root_group/myawesomeproject'",
			},
			false,
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
			false,
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
			false,
		},
		{
			"share other_group group with root_group group as developers",
			"fixtures/plain.yaml",
			"fixtures/diff-share-other_group-group-with-root_group-group-as-developer.yaml",
			[]string{
				"share group 'root_group' with group 'other_group' at level 'Developer'",
			},
			false,
		},
		{
			"add project permissions",
			"fixtures/plain.yaml",
			"fixtures/plain-with-project.yaml",
			[]string{
				"share project 'root_group/a_project' with group 'other_group' at level 'Developer'",
				"add 'admin' to 'root_group/a_project' at level 'Maintainer'",
				"add 'user2' to 'root_group/a_project' at level 'Developer'",
			},
			false,
		},
		{
			"change project permissions",
			"fixtures/plain-with-project.yaml",
			"fixtures/plain-with-other-levels-project.yaml",
			[]string{
				"add 'user1' to 'root_group/a_project' at level 'Developer'",
				"change 'user2' in 'root_group/a_project' to level 'Maintainer'",
				"add 'user3' to 'root_group/a_project' at level 'Reporter'",
				"remove project sharing from 'root_group/a_project' with group 'other_group'",
			},
			false,
		},
		{
			"change project permissions the other way",
			"fixtures/plain-with-other-levels-project.yaml",
			"fixtures/plain-with-project.yaml",
			[]string{
				"share project 'root_group/a_project' with group 'other_group' at level 'Developer'",
				"change 'user2' in 'root_group/a_project' to level 'Developer'",
				"remove 'user1' from 'root_group/a_project'",
				"remove 'user3' from 'root_group/a_project'",
			},
			false,
		},
		{
			"plain project permissions without changes",
			"fixtures/plain-with-project.yaml",
			"fixtures/plain-with-project.yaml",
			[]string{},
			true,
		},
		{
			"blocking a user works",
			"fixtures/plain-with-admins.yaml",
			"fixtures/plain-with-blocked-user.yaml",
			[]string{
				"unset 'user3' as admin",
				"remove 'user3' from 'other_group'",
				"block 'user3'",
			},
			true,
		},
		{
			"unblocking a user works",
			"fixtures/plain-with-blocked-user.yaml",
			"fixtures/plain-with-admins.yaml",
			[]string{
				"unblock 'user3'",
				"set 'user3' as admin",
				"add 'user3' to 'other_group' at level 'Developer'",
			},
			true,
		},
		{
			"removing a user happens in order",
			"fixtures/diff-root-with-multi-level-user.yaml",
			"fixtures/diff-root-with-multi-level-admin.yaml",
			[]string{
				"remove 'user1' from 'root_group/subgroup1'",
				"remove 'user1' from 'root_group/subgroup2'",
				"remove 'user1' from 'root_group'",
				"remove 'user1' from 'root_group/a_project'",
			},
			true,
		},
		{
			"adding a bot works",
			"fixtures/plain-minimal.yaml",
			"fixtures/plain-bots.yaml",
			[]string{
				"create bot user 'bot1' with email 'bot@bot.com",
			},
			true,
		},
		{
			"changing a bot email",
			"fixtures/plain-bots.yaml",
			"fixtures/plain-bots-with-other-email.yaml",
			[]string{
				"update bot 'bot1' email to 'bot@my-own-domain-with-blackjack-and-hookers.com'",
			},
			true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			// if tc.desiredState == "fixtures/diff-share-other_group-group-with-root_group-group-as-developer.yaml" {
			//     logrus.SetLevel(logrus.DebugLevel)
			// } else {
			//     logrus.SetLevel(logrus.InfoLevel)
			// }

			sourceConfig, err := util.LoadConfig(tc.sourceState, false)
			a.NoError(err, "source config")

			sourceState, err := state.LoadStateFromFile(sourceConfig, querier)
			a.NoError(err, "source state")

			desiredConfig, err := util.LoadConfig(tc.desiredState, false)
			a.NoError(err, "desired config")

			desiredState, err := state.LoadStateFromFile(desiredConfig, querier)
			a.NoError(err, "desired state")

			actions, err := state.Diff(sourceState, desiredState, state.DiffArgs{
				DiffGroups:   true,
				DiffProjects: true,
				DiffUsers:    true,
				DiffBots:     true,
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

			a.Equal(len(tc.desiredActions), len(executedActions), "actions length is not as expected")
			// a.Equal(tc.desiredActions, executedActions, "actions are not as expected")

			logrus.Debugf("Executed: %v", executedActions)
			logrus.Debugf("Desired %v", tc.desiredActions)
			if tc.inOrder {
				a.EqualValues(tc.desiredActions, executedActions, "actions")
			} else {
				for _, action := range tc.desiredActions {
					a.Contains(executedActions, action, "more desired actions than executed")
				}
				for _, action := range executedActions {
					a.Contains(tc.desiredActions, action, "more executed actions than desired")
				}
			}

		})
	}
}

func TestLoadingStateWithoutEnvironmentSetFails(t *testing.T) {
	a := assert.New(t)

	desiredConfig, err := util.LoadConfig("fixtures/plain-with-project-with-secrets.yaml", false)
	a.NoError(err, "desired config")

	_, err = state.LoadStateFromFile(desiredConfig, querier)
	a.EqualError(err, "failed to build local state: 2 errors: "+
		"Group contains secret 'mygroupkey'='myenvgroupkey' which is not loaded in the environment; "+
		"Project contains secret 'mykey'='myenvkey' which is not loaded in the environment")
}

func TestDiffingVariablesWorksAsExpected(t *testing.T) {
	tt := []struct {
		name           string
		sourceState    string
		desiredState   string
		desiredActions []string
		environment    map[string]string
		yolo           bool
		expectedError  string
	}{
		{
			"create a set of variables in new groups",
			"fixtures/plain-minimal.yaml",
			"fixtures/plain-with-project-with-secrets.yaml",
			[]string{
				"create group variable 'mygroupkey' in 'other_group'",
				"create project variable 'mykey' in 'root_group/a_project'",
				"add 'admin' to 'root_group/a_project' at level 'Maintainer'",
				"add 'user2' to 'other_group' at level 'Owner'",
			},
			map[string]string{
				"myenvkey":      "value",
				"myenvgroupkey": "othervalue",
			},
			false,
			"",
		},
		{
			"create a set of variables",
			"fixtures/plain-with-project-without-variables.yaml",
			"fixtures/plain-with-project-with-secrets.yaml",
			[]string{
				"create group variable 'mygroupkey' in 'other_group'",
				"create project variable 'mykey' in 'root_group/a_project'",
			},
			map[string]string{
				"myenvkey":      "value",
				"myenvgroupkey": "othervalue",
			},
			false,
			"",
		},
		{
			"variables are the same",
			"fixtures/plain-with-project-with-secrets.yaml",
			"fixtures/plain-with-project-with-other-secrets.yaml",
			[]string{},
			map[string]string{
				"myenvkey":           "value",
				"myenvgroupkey":      "othervalue",
				"myotherenvkey":      "value",
				"myotherenvgroupkey": "othervalue",
			},
			true,
			"",
		},
		{
			"update a set of variables",
			"fixtures/plain-with-project-with-secrets.yaml",
			"fixtures/plain-with-project-with-other-secrets.yaml",
			[]string{
				"update group variable 'mygroupkey' in 'other_group'",
				"update project variable 'mykey' in 'root_group/a_project'",
			},
			map[string]string{
				"myenvkey":           "value",
				"myenvgroupkey":      "othervalue",
				"myotherenvkey":      "othervalue",
				"myotherenvgroupkey": "yetanothervalue",
			},
			true,
			"",
		},
		{
			"update a set of variables without yolo mode fails",
			"fixtures/plain-with-project-with-secrets.yaml",
			"fixtures/plain-with-project-with-other-secrets.yaml",
			[]string{
				"update group variable 'mygroupkey' in 'other_group'",
				"update project variable 'mykey' in 'root_group/a_project'",
			},
			map[string]string{
				"myenvkey":           "value",
				"myenvgroupkey":      "othervalue",
				"myotherenvkey":      "othervalue",
				"myotherenvgroupkey": "yetanothervalue",
			},
			false,
			"2 errors: variable mygroupkey in group other_group is not as expected; " +
				"variable mykey in project root_group/a_project is not as expected",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			if tc.environment != nil {
				for k, v := range tc.environment {
					a.NoError(os.Setenv(k, v))
					defer os.Setenv(k, "")
				}
			}

			sourceConfig, err := util.LoadConfig(tc.sourceState, false)
			a.NoError(err, "source config")

			sourceState, err := state.LoadStateFromFile(sourceConfig, querier)
			a.NoError(err, "source state")

			desiredConfig, err := util.LoadConfig(tc.desiredState, false)
			a.NoError(err, "desired config")

			desiredState, err := state.LoadStateFromFile(desiredConfig, querier)
			a.NoError(err, "desired state")

			actions, err := state.Diff(sourceState, desiredState, state.DiffArgs{
				DiffGroups:   true,
				DiffProjects: true,
				DiffUsers:    true,
				Yolo:         tc.yolo,
			})
			if err != nil {
				a.EqualError(err, tc.expectedError)
				return // Error was expected, get out
			}

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

			a.Equal(tc.desiredActions, executedActions, "actions are not as expected")

			for _, action := range tc.desiredActions {
				a.Contains(executedActions, action, "more desired actions than executed")
			}
			for _, action := range executedActions {
				a.Contains(tc.desiredActions, action, "more executed actions than desired")
			}
		})
	}
}
