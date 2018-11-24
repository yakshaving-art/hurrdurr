package main

import (
	"github.com/onrik/logrus/filename"
	"github.com/sirupsen/logrus"
)

func main() {
	setupLogger()

	args := parseArgs()

	load(args.GitlabToken, args.GitlabBaseURL)

	if args.DryRun {
		// New implementation goes here
		logrus.Infof("Not implemented yet")

	} else {
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
