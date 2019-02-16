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

	GhostUser string

	DryRun        bool
	ShowVersion   bool
	Debug         bool
	ChecksumCheck bool

	ManageACLs  bool
	ManageUsers bool

	AutoDevOpsMode bool
	YoloMode       bool

	SnoopDepth int
}

func parseArgs() Args {
	args := Args{}

	flag.BoolVar(&args.ShowVersion, "version", false, "show version and exit")
	flag.BoolVar(&args.DryRun, "dryrun", false, "executes in dryrun mode. Avoids making any change")
	flag.BoolVar(&args.Debug, "debug", false, "executes with logging in debug mode")
	flag.BoolVar(&args.ChecksumCheck, "checksum-check", false, "validates the configuration checksum "+
		"reading it from a file called as the configuratio file ended in .md5")

	flag.StringVar(&args.ConfigFile, "config", "config.yaml", "configuration file to load")
	flag.StringVar(&args.GhostUser, "ghost-user", "ghost", "system wide gitlab ghost user.")

	flag.BoolVar(&args.ManageACLs, "manage-acls", false, "performs diffs of groups and projects")
	flag.BoolVar(&args.ManageUsers, "manage-users", false, "performs diffs of user attributes")
	flag.BoolVar(&args.AutoDevOpsMode, "autodevopsmode", false,
		"where you have no admin rights but still do what you gotta do")
	flag.BoolVar(&args.YoloMode, "yolo-force-secrets-overwrite", false,
		"life is too short to not overwrite group and project environment variables")
	flag.IntVar(&args.SnoopDepth, "snoopdepth", 0, "max depth to report unhandled groups. 0 means all")

	flag.Parse()

	args.GitlabToken = os.Getenv("GITLAB_TOKEN")
	args.GitlabBaseURL = os.Getenv("GITLAB_BASEURL")

	if args.ShowVersion {
		logrus.Printf(version.GetVersion())
		os.Exit(0)
	}

	if args.GitlabToken == "" {
		logrus.Fatal("GITLAB_TOKEN is a required environment variable")
	}

	if args.GitlabBaseURL == "" {
		logrus.Fatal("GITLAB_BASEURL is a required environment variable")
	}

	if !strings.HasPrefix(args.GitlabBaseURL, "https://") {
		logrus.Fatal("Validate error: base_url should use https:// scheme")
	}
	if !strings.HasSuffix(args.GitlabBaseURL, "/api/v4/") {
		logrus.Fatal("Validate error: base_url should end with '/api/v4/'")
	}

	if !(args.ManageACLs || args.ManageUsers) {
		logrus.Fatal("Nothing to manage, set one of -manage-acls or -manage-users")
	}

	return args
}
