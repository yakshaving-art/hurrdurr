package main

import (
	"fmt"
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

	if args.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetFormatter(&logrus.TextFormatter{
			DisableTimestamp: true,
		})
		logrus.Debugf("Enabling debug level logging, with timestamps")
	}

	conf, err := util.LoadConfig(args.ConfigFile, args.ChecksumCheck)
	if err != nil {
		logrus.Fatalf("failed to load configuration: %s", err)
	}
	logrus.Debugf("Configuration loaded from file %s", args.ConfigFile)

	if args.ManageBots {
		if err := util.ValidateBots(conf.Bots, args.BotUsernameRegex); err != nil {
			logrus.Fatalf("Failed validating bots users: %s", err)
		}
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

		logrus.Debugf("Loaded partial state from gitlab")
	} else {
		err := api.CreatePreloadedQuerier(&client)
		if err != nil {
			logrus.Fatalf("Failed to preload querier from gitlab instance: %s", err)
		}

		currentState, err = api.LoadFullGitlabState(client)
		if err != nil {
			logrus.Fatalf("Failed to load full live state from gitlab instance: %s", err)
		}

		logrus.Debugf("Loaded full state from gitlab")
	}

	desiredState, err := state.LoadStateFromFile(conf, client.Querier)
	if err != nil {
		logrus.Fatalf("Failed to load desired state from file %s: %s", args.ConfigFile, err)
	}

	logrus.Debugf("Loaded desired state from file %s", args.ConfigFile)

	actions, err := state.Diff(currentState, desiredState, state.DiffArgs{
		DiffGroups:   args.ManageACLs,
		DiffProjects: args.ManageACLs,
		DiffUsers:    args.ManageUsers,
		DiffBots:     args.ManageBots,

		Yolo: args.YoloMode,
	})
	if err != nil {
		logrus.Fatalf("Failed to diff current and desired state: %s", err)
	}

	logrus.Debugf("Diff calculated")

	var actionClient internal.APIClient

	if args.DryRun {
		fmt.Println("Changes proposed [dryrun]:")
		actionClient = api.DryRunAPIClient{
			Append: func(change string) {
				fmt.Printf("  %s\n", change)
			},
		}
	} else {
		fmt.Println("Executing Changes:")
		actionClient = client
	}

	if len(actions) == 0 {
		fmt.Println("  No changes necessary")
	}
	for _, action := range actions {
		if err := action.Execute(actionClient); err != nil {
			logrus.Fatalf("Faile to run action: %s", err)
		}
	}

	logrus.Debugf("All actions executed")

	if len(desiredState.UnhandledGroups()) > 0 {
		fmt.Println("Unhandled groups detected:")
		for _, ug := range desiredState.UnhandledGroups() {
			if args.SnoopDepth == 0 || strings.Count(ug, "/") <= args.SnoopDepth {
				logrus.Infof("  %s", ug)
			}
		}
	}

	logrus.Debugf("Done")
}
