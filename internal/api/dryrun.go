package api

import "fmt"

// DryRunAPIClient provides a simple interface that will send any section the
// embedded Append function
type DryRunAPIClient struct {
	Append func(string)
}

// AddMembership implements the APIClient interface
func (m DryRunAPIClient) AddMembership(username, group string, level int) {
	m.Append(fmt.Sprintf("add '%s' to '%s' at level '%d'", username, group, level))
}

// ChangeMembership implements the APIClient interface
func (m DryRunAPIClient) ChangeMembership(username, group string, level int) {
	m.Append(fmt.Sprintf("change '%s' to '%s' at level '%d'", username, group, level))
}

// RemoveMembership implements the APIClient interface
func (m DryRunAPIClient) RemoveMembership(username, group string) {
	m.Append(fmt.Sprintf("remove '%s' from '%s'", username, group))

}
