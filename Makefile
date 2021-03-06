# Makefile for gitlab group manager
# vim: set ft=make ts=8 noet
# Copyright Yakshaving.art
# Licence MIT

# Variables
# UNAME		:= $(shell uname -s)

COMMIT_ID := `git log -1 --format=%H`
COMMIT_DATE := `git log -1 --format=%aI`
VERSION := $${CI_COMMIT_TAG:-SNAPSHOT-$(COMMIT_ID)}

# this is godly
# https://news.ycombinator.com/item?id=11939200
help:	### this screen. Keep it first target to be default
ifeq ($(UNAME), Linux)
	@grep -P '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
else
	@# this is not tested, but prepared in advance for you, Mac drivers
	@awk -F ':.*###' '$$0 ~ FS {printf "%15s%s\n", $$1 ":", $$2}' $(MAKEFILE_LIST) | grep -v '@awk' | sort
endif

.PHONY: help debug check test build 

# Targets
#
debug:	### Debug Makefile itself
	@echo $(UNAME)

check:	### Sanity checks
	@find . -type f \( -name \*.yml -o -name \*yaml \) \! -path './vendor/*' | xargs -r yq '.' # >/dev/null

test:	### run all the tests
	go test -v -coverprofile=coverage.out $$(go list ./... | grep -v '/vendor/') && go tool cover -func=coverage.out

build:  ### build the binary
	@go build -ldflags "-X gitlab.com/yakshaving.art/hurrdurr/version.Version=$(VERSION) -X gitlab.com/yakshaving.art/hurrdurr/version.Commit=$(COMMIT_ID) -X gitlab.com/yakshaving.art/hurrdurr/version.Date=$(COMMIT_DATE)"
	@strip hurrdurr
