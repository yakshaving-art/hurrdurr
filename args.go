package main

import (
	"flag"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.com/yakshaving.art/hurrdurr/version"
)

// Args is used to load all the flags and arguments provided by the user
type Args struct {
	ConfigFile string

	GitlabToken   string
	GitlabBaseURL string

	DryRun      bool
	ShowVersion bool
	Debug       bool
}

func parseArgs() Args {
	args := Args{}

	flag.BoolVar(&args.ShowVersion, "version", false, "show version and exit")
	flag.BoolVar(&args.DryRun, "dryrun", false, "executes in dryrun mode. Avoids making any change")
	flag.BoolVar(&args.Debug, "debug", false, "executes with logging in debug mode")

	flag.StringVar(&args.ConfigFile, "config", "config.yaml", "configuration file to load")

	flag.Parse()

	args.GitlabToken = os.Getenv("GITLAB_TOKEN")
	args.GitlabBaseURL = os.Getenv("GITLAB_BASEURL")

	if args.ShowVersion {
		logrus.Printf(version.GetVersion())
		os.Exit(0)
	}

	if args.GitlabToken == "" {
		logrus.Fatalf("GITLAB_TOKEN is a required environment variable")
	}

	if args.GitlabBaseURL == "" {
		logrus.Fatalf("GITLAB_BASEURL is a required environment variable")
	}

	if !strings.HasPrefix(args.GitlabBaseURL, "https://") {
		logrus.Fatalf("Validate error: base_url should use https:// scheme")
	}
	if !strings.HasSuffix(args.GitlabBaseURL, "/api/v4/") {
		logrus.Fatalf("Validate error: base_url should end with '/api/v4/'")
	}

	return args
}
