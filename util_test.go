package main

import (
	"os"
	"testing"
)

func TestReadFromEnv(t *testing.T) {
	varName := "HURR_DURR_TEST_VAR"
	varValue := "somevar"

	os.Setenv(varName, varValue)
	defer os.Unsetenv(varName)

	result := readFromEnv(varName)

	if result != varValue {
		t.Errorf("Got '%s', expected '%s'", result, varValue)
	}
}
