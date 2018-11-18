package main

import (
	"flag"
	"log"
	"os"

	"gitlab.com/yakshaving.art/hurrdurr/version"
)

func main() {
	showVersion := flag.Bool("version", false, "show version and exit")
	configFile := flag.String("config", "config.yaml", "configuration file to load")

	flag.Parse()

	if *showVersion {
		log.Printf(version.GetVersion())
		os.Exit(0)
	}

	load()

	cfg.read(*configFile)
	cfg.validate()
	cfg.apply()
	cfg.report()
}
