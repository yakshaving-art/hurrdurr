package main

import (
	"flag"
	"log"
	"os"

	"gitlab.com/yakshaving.art/hurrdurr/version"
)

func main() {
	showVersion := flag.Bool("version", false, "show version and exit")

	flag.Parse()

	if *showVersion {
		log.Printf("Version: %s Commit: %s Date: %s", version.Version, version.Commit, version.Date)
		os.Exit(0)
	}

	load()

	cfg.read("config.yaml")
	cfg.validate()
	cfg.apply()
	cfg.report()
}
