package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/util"
)

func TestLoadingValidMD5Check(t *testing.T) {
	a := assert.New(t)
	c, err := util.LoadConfig("fixtures/config-sample.yml", true)

	a.NoError(err)
	a.EqualValues(internal.Config{
		Groups: map[string]internal.Acls{
			"yakshavers": {
				Owners: []string{"root"},
			},
		},
		Projects: map[string]internal.Acls{
			"someproject": {
				Owners: []string{"root"},
			},
		},
		Users: internal.Users{
			Admins:  []string{"root"},
			Blocked: []string{"bad_actor"},
		},
		Bots: []internal.Bot{
			internal.Bot{
				Username: "bot_one",
				Email:    "bot@bot.com",
			},
		},
	}, c)
}

func TestLoadingMultifileConfig(t *testing.T) {
	a := assert.New(t)
	c, err := util.LoadConfig("fixtures/multifile-config.yml", true)

	a.NoError(err)
	a.EqualValues(internal.Config{
		Groups: map[string]internal.Acls{
			"yakshavers": {
				Owners: []string{"root"},
			},
		},
		Projects: map[string]internal.Acls{
			"myproject": {
				Owners: []string{"me"},
			},
			"someproject": {
				Owners: []string{"root"},
			},
		},
		Users: internal.Users{
			Admins:  []string{"root"},
			Blocked: []string{"bad_actor"},
		},
		Files: []string{"fixtures/config-sample.yml"},
		Bots: []internal.Bot{
			internal.Bot{
				Username: "bot_one",
				Email:    "bot@bot.com",
			},
		},
	}, c)
}

func TestLoadingInvalidConfig(t *testing.T) {
	a := assert.New(t)
	_, err := util.LoadConfig("fixtures/invalid-config-sample.yml", false)

	a.EqualError(err, "failed to unmarshal state file fixtures/invalid-config-sample.yml: yaml: unmarshal errors:\n"+
		"  line 3: field not-valid-key not found in type internal.Config")
}

func TestLoadingNonExistingConfig(t *testing.T) {
	a := assert.New(t)
	_, err := util.LoadConfig("fixtures/non-existing-config.yml", true)

	a.EqualError(err, "failed to load state file fixtures/non-existing-config.yml: "+
		"open fixtures/non-existing-config.yml: no such file or directory")
}
func TestLoadingValidWithoutMD5Check(t *testing.T) {
	a := assert.New(t)
	c, err := util.LoadConfig("fixtures/config-without-md5.yml", false)

	a.NoError(err)
	a.EqualValues(internal.Config{
		Groups:   map[string]internal.Acls{},
		Projects: map[string]internal.Acls{},
		Users: internal.Users{
			Admins:  []string{},
			Blocked: []string{},
		},
		Bots: []internal.Bot{},
	}, c)
}

func TestLoadingInvalidMD5Check(t *testing.T) {

	a := assert.New(t)
	_, err := util.LoadConfig("fixtures/config-wrong-md5.yml", true)

	a.EqualError(err, "configuration file calculated md5 '671cc46a43b1632047ba2677a51f8b58' "+
		"does not match the provided md5 ' 671cc46a43b1632047ba2677a51f8b58'")
}

func TestToStringSlice(t *testing.T) {
	s := util.ToStringSlice(map[string]int{
		"a": 0,
		"b": 1,
		"c": 2,
	})

	a := assert.New(t)
	a.EqualValues(s, []string{"a", "b", "c"})
}

func TestToStringSliceIgnoring(t *testing.T) {
	s := util.ToStringSliceIgnoring(map[string]int{
		"a": 0,
		"b": 1,
		"c": 2,
	}, "c")

	a := assert.New(t)
	a.EqualValues(s, []string{"a", "b"})
}

func TestBotUsernames(t *testing.T) {
	a := assert.New(t)

	a.NoError(util.ValidateBots([]internal.Bot{
		{Username: "bot1", Email: "myemail"},
	}, "^bot.+$"))

	a.EqualError(util.ValidateBots([]internal.Bot{
		{Username: "nobot1", Email: "myemail"},
	}, "^bot.+$"), "invalid bot username nobot1")

	a.EqualError(util.ValidateBots([]internal.Bot{
		{Username: "bot1", Email: ""},
	}, "^bot.+$"), "bot bot1 has an empty email")
}
