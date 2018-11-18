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
			"failed to load state file : open : no such file or directory",
			nil,
		},
		{
			"plain state",
			"fixtures/plain.yaml",
			"",
			[]hurrdurr.Group{
				{
					Fullpath: "root_group",
					Members: []hurrdurr.Membership{
						{
							Username: "user1",
							Level:    hurrdurr.Developer,
						},
						{
							Username: "admin",
							Level:    hurrdurr.Owner,
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			s, err := hurrdurr.LoadStateFromFile(tc.stateFile)
			if tc.expectedError != "" {
				a.EqualErrorf(err, tc.expectedError, "Wrong error, expected %s, got %s", tc.expectedError, err)
				return
			}

			if err != nil {
				t.Fatalf("failed to read fixture file %s: %s", tc.stateFile, err)
			}
			a.Equalf(tc.expected, s.Groups(), "Wrong state, groups are not as expected")
		})
	}
}
