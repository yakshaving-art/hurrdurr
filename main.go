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
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableTimestamp: true,
	})

	args := parseArgs()

	SetupLogger(args.Debug, args.Trace)

	conf, err := util.LoadConfig(args.ConfigFile, args.ChecksumCheck)
	if err != nil {
		logrus.Fatalf("failed to load configuration: %s", err)
	}
	logrus.Debugf("configuration loaded from file %s", args.ConfigFile)

	if args.ManageBots {
		if err := util.ValidateBots(conf.Bots, args.BotUsernameRegex); err != nil {
			logrus.Fatalf("failed validating bots users: %s", err)
		}
	}

	client := api.NewGitlabAPIClient(
		api.GitlabAPIClientArgs{
			GitlabToken:     args.GitlabToken,
			GitlabBaseURL:   args.GitlabBaseURL,
			GitlabGhostUser: args.GhostUser,
			Concurrency:     args.Concurrency,
		})

	var currentState internal.State
	if args.AutoDevOpsMode {
		logrus.Infof("loading partial state from gitlab")
		err := api.CreateLazyQuerier(&client)
		if err != nil {
			logrus.Fatalf("failed to create lazy querier from gitlab instance: %s", err)
		}

		currentState, err = api.LoadPartialGitlabState(conf, client)
		if err != nil {
			logrus.Fatalf("failed to load partial live state from gitlab instance: %s", err)
		}

		logrus.Infof("done loading partial state from gitlab")
	} else {
		logrus.Infof("loading full state from gitlab")
		err := api.CreatePreloadedQuerier(&client)
		if err != nil {
			logrus.Fatalf("failed to preload querier from gitlab instance: %s", err)
		}

		currentState, err = api.LoadFullGitlabState(client)
		if err != nil {
			logrus.Fatalf("failed to load full live state from gitlab instance: %s", err)
		}

		logrus.Infof("done loading full state from gitlab")
	}

	desiredState, err := state.LoadStateFromFile(conf, client.Querier)
	logrus.Infof("loading desired state from file %s", args.ConfigFile)
	if err != nil {
		logrus.Fatalf("failed to load desired state from file %s: %s", args.ConfigFile, err)
	}

	logrus.Infof("done loading desired state from file %s", args.ConfigFile)

	actions, err := state.Diff(currentState, desiredState, state.DiffArgs{
		DiffGroups:   args.ManageACLs,
		DiffProjects: args.ManageACLs,
		DiffUsers:    args.ManageUsers,
		DiffBots:     args.ManageBots,

		Yolo: args.YoloMode,
	})
	if err != nil {
		logrus.Fatalf("failed to diff current and desired state: %s", err)
	}

	logrus.Debugf("diff calculated")

	var actionClient internal.APIClient

	if args.DryRun {
		logrus.Println("changes proposed [dryrun]:")
		actionClient = api.DryRunAPIClient{
			Append: func(change string) {
				logrus.Printf("  %s", change)
			},
		}
	} else {
		logrus.Print("executing changes:")
		actionClient = client
	}

	if len(actions) == 0 {
		logrus.Print("  no changes necessary")
	}
	for _, action := range actions {
		if err := action.Execute(actionClient); err != nil {
			logrus.Fatalf("Failed to run action: %s", err)
		}
	}

	logrus.Debugf("all actions executed")

	if len(desiredState.UnhandledGroups()) > 0 {
		logrus.Print("unhandled groups detected:")
		for _, ug := range desiredState.UnhandledGroups() {
			if args.SnoopDepth == 0 || strings.Count(ug, "/") <= args.SnoopDepth {
				logrus.Infof("  %s", ug)
			}
		}
	}

	logrus.Infof("done")
}
