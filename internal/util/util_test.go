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
	a.EqualValues(internal.Config{}, c)
}

func TestLoadingInvalidConfig(t *testing.T) {
	a := assert.New(t)
	_, err := util.LoadConfig("fixtures/invalid-config-sample.yml", false)

	a.EqualError(err, "failed to unmarshal state file fixtures/invalid-config-sample.yml: yaml: unmarshal errors:\n"+
		"  line 4: field not-valid-key not found in type internal.Config")
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
	a.EqualValues(internal.Config{}, c)
}

func TestLoadingInvalidMD5Check(t *testing.T) {

	a := assert.New(t)
	_, err := util.LoadConfig("fixtures/config-wrong-md5.yml", true)

	a.EqualError(err, "configuration file calculated md5 'dbc6d0334ddedc38552cdd19cdbd83a3' "+
		"does not match the provided md5 ' dbc6d0334ddedc38552cdd19cdbd83aa'")
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
