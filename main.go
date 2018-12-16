package main

import (
	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/api"
	"gitlab.com/yakshaving.art/hurrdurr/internal/state"

	"github.com/sirupsen/logrus"
)

func main() {
	args := parseArgs()

	if args.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	client := api.NewGitlabAPIClient(
		api.GitlabAPIClientArgs{
			GitlabToken:     args.GitlabToken,
			GitlabBaseURL:   args.GitlabBaseURL,
			GitlabGhostUser: args.GhostUser,
		})

	err := client.CreatePreloadedQuerier()
	if err != nil {
		logrus.Fatalf("Failed to preload querier from gitlab instance: %s", err)
	}

	gitlabState, err := client.LoadGitlabState()
	if err != nil {
		logrus.Fatalf("Failed to load live state from gitlab instance: %s", err)
	}

	desiredState, err := state.LoadStateFromFile(args.ConfigFile, client.Querier)
	if err != nil {
		logrus.Fatalf("Failed to load desired state from file %s: %s", args.ConfigFile, err)
	}

	actions, err := state.Diff(gitlabState, desiredState, state.DiffArgs{
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
				logrus.Info(change)
			},
		}
	} else {
		actionClient = client
	}

	for _, action := range actions {
		if err := action.Execute(actionClient); err != nil {
			logrus.Fatalf("Faile to run action: %s", err)
		}
	}

	if len(desiredState.UnhandledGroups()) > 0 {
		logrus.Infof("Unhandled groups detected:")
		for _, ug := range desiredState.UnhandledGroups() {
			logrus.Infof("  %s", ug)
		}
	}
}
