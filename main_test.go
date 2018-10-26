package main

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	c := config{}
	c.read("./test_data/config.yaml")
	if c.Groups[0]["group1"][0].AccessLevel != "owner" {
		t.Errorf("wrong data structure")
	}
}
