package internal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	hurrdurr "gitlab.com/yakshaving.art/hurrdurr/internal"
)

func TestLoadingState(t *testing.T) {
	tt := []struct {
		name          string
		stateFile     string
		expectedError string
		expected      []hurrdurr.Group
	}{
		{
			"non existing file",
			"",
			"Failed to load state file : open : no such file or directory",
			nil,
		},
		// { Next test
		// 	"plain state",
		// 	"plain.yaml",
		// 	"",
		// 	[]hurrdurr.Group{},
		// },
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			s, err := hurrdurr.LoadStateFromFile(tc.stateFile)
			if tc.expectedError != "" {
				a.EqualErrorf(err, tc.expectedError, "Wrong error, expected %s, got %s", tc.expectedError, err)
				return
			}

			a.Nilf(err, "failed to read fixture file %s", tc.stateFile)
			a.NotNilf(s, "file %s returned a nil state", tc.stateFile)
			a.Equalf(tc.expected, s.Groups(), "Wrong state, expected %#v, got %#v", tc.expected, s)
		})
	}
}
