package main

import "github.com/sirupsen/logrus"

func main() {
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
