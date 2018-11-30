package main

import (
	"gitlab.com/yakshaving.art/hurrdurr/internal/api"
	"gitlab.com/yakshaving.art/hurrdurr/internal/state"

	"github.com/onrik/logrus/filename"
	"github.com/sirupsen/logrus"
)

func main() {
	setupLogger()

	args := parseArgs()

	if args.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if args.DryRun {
		// When we're comfortable that it works ok, we can move this logic out of the
		// if so it happens all the time (it's the core algorithm) then all the change
		// to either apply the changes or not is to use the real gitlabClient or the
		// dryrun one
		client := api.NewGitlabAPIClient(args.GitlabToken, args.GitlabBaseURL)

		gitlabQuerier, gitlabState, err := client.LoadState()
		if err != nil {
			logrus.Fatalf("Failed to load live state from gitlab instance: %s", err)
		}

		desiredState, err := state.LoadStateFromFile(args.ConfigFile, gitlabQuerier)
		if err != nil {
			logrus.Fatalf("Failed to load desired state from file %s: %s", args.ConfigFile, err)
		}

		actions, err := state.Diff(gitlabState, desiredState)
		if err != nil {
			logrus.Fatalf("Failed to diff current and desired state: %s", err)
		}

		dryrunClient := api.DryRunAPIClient{
			Append: func(change string) {
				logrus.Info(change)
			},
		}

		for _, action := range actions {
			if err := action.Execute(dryrunClient); err != nil {
				logrus.Fatalf("Faile to run action: %s", err)
			}
		}

	} else {
		load(args.GitlabToken, args.GitlabBaseURL)

		cfg.read(args.ConfigFile)
		cfg.validate()
		cfg.apply()
		cfg.report()
	}
}

func setupLogger() {
	logrus.AddHook(filename.NewHook())
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
}
