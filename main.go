package main

import (
	"strings"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/api"
	"gitlab.com/yakshaving.art/hurrdurr/internal/state"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"

	"github.com/sirupsen/logrus"
)

func main() {
	args := parseArgs()

	if args.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	conf, err := util.LoadConfig(args.ConfigFile)
	if err != nil {
		logrus.Fatalf("failed to load configuration: %s", err)
	}

	client := api.NewGitlabAPIClient(
		api.GitlabAPIClientArgs{
			GitlabToken:     args.GitlabToken,
			GitlabBaseURL:   args.GitlabBaseURL,
			GitlabGhostUser: args.GhostUser,
		})

	var currentState internal.State
	if args.AutoDevOpsMode {
		err := api.CreateLazyQuerier(&client)
		if err != nil {
			logrus.Fatalf("Failed to create lazy querier from gitlab instance: %s", err)
		}

		currentState, err = api.LoadPartialGitlabState(conf, client)
		if err != nil {
			logrus.Fatalf("Failed to load partial live state from gitlab instance: %s", err)
		}

	} else {
		err := api.CreatePreloadedQuerier(&client)
		if err != nil {
			logrus.Fatalf("Failed to preload querier from gitlab instance: %s", err)
		}

		currentState, err = api.LoadFullGitlabState(client)
		if err != nil {
			logrus.Fatalf("Failed to load full live state from gitlab instance: %s", err)
		}
	}

	desiredState, err := state.LoadStateFromFile(conf, client.Querier)
	if err != nil {
		logrus.Fatalf("Failed to load desired state from file %s: %s", args.ConfigFile, err)
	}

	actions, err := state.Diff(currentState, desiredState, state.DiffArgs{
		DiffGroups:   args.ManageACLs,
		DiffProjects: args.ManageACLs,
		DiffUsers:    args.ManageUsers,
	})
	if err != nil {
		logrus.Fatalf("Failed to diff current and desired state: %s", err)
	}

	var actionClient internal.APIClient

	if args.DryRun {
		actionClient = api.DryRunAPIClient{
			Append: func(change string) {
				logrus.Info("  ", change)
			},
		}
	} else {
		actionClient = client
	}

	logrus.Infof("Changes:")
	if len(actions) == 0 {
		logrus.Infof("  No changes necessary")
	}
	for _, action := range actions {
		if err := action.Execute(actionClient); err != nil {
			logrus.Fatalf("Faile to run action: %s", err)
		}
	}

	if len(desiredState.UnhandledGroups()) > 0 {
		logrus.Infof("Unhandled groups detected:")
		for _, ug := range desiredState.UnhandledGroups() {
			if args.SnoopDepth == 0 || strings.Count(ug, "/") <= args.SnoopDepth {
				logrus.Infof("  %s", ug)
			}
		}
	}
}
