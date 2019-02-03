package util

import (
	"fmt"
	"io/ioutil"
	"sort"

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
func LoadConfig(filename string) (internal.Config, error) {
	c := internal.Config{}

	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return c, fmt.Errorf("failed to load state file %s: %s", filename, err)
	}

	if err := yaml.UnmarshalStrict(content, &c); err != nil {
		return c, fmt.Errorf("failed to unmarshal state file %s: %s", filename, err)
	}
	return c, nil
}
