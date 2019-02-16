package util

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	"gitlab.com/yakshaving.art/hurrdurr/internal"

	yaml "gopkg.in/yaml.v2"
)

// ToStringSlice turns a map[string]int into a []string
func ToStringSlice(m map[string]int) []string {
	slice := make([]string, 0)
	for v := range m {
		slice = append(slice, v)
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
	return slice
}

// ToStringSliceIgnoring turns a map[string]int into a []string, ignoring `ignore` values
func ToStringSliceIgnoring(m map[string]int, ignore string) []string {
	slice := make([]string, 0)
	for v := range m {
		if v == ignore {
			continue
		}

		slice = append(slice, v)
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
	return slice
}

// LoadConfig reads the given filename and parses it into a config struct
func LoadConfig(filename string, checksumCheck bool) (internal.Config, error) {
	c := internal.Config{}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return c, fmt.Errorf("failed to load state file %s: %s", filename, err)
	}

	if checksumCheck {
		md5hash, err := ioutil.ReadFile(fmt.Sprintf("%s.md5", filename))
		if err != nil {
			return c, fmt.Errorf("failed to read checksum configuration file: %s", err)
		}

		m := md5.New()
		m.Write(content)
		calculatedMD5 := hex.EncodeToString(m.Sum(nil))
		if strings.TrimSpace(string(md5hash)) != calculatedMD5 {
			return c, fmt.Errorf("configuration file calculated md5 '%s' does not match the provided md5 '%s'", calculatedMD5, md5hash)
		}
		logrus.Info("configuration md5 sum validated correctly")
	}

	if err := yaml.UnmarshalStrict(content, &c); err != nil {
		return c, fmt.Errorf("failed to unmarshal state file %s: %s", filename, err)
	}
	return c, nil
}
