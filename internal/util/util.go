package util

import (
	"sort"
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
