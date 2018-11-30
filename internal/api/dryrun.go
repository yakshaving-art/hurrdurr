package api

import (
	"fmt"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
)

// DryRunAPIClient provides a simple interface that will send any section the
// embedded Append function
type DryRunAPIClient struct {
	Append func(string)
}

// AddMembership implements the APIClient interface
func (m DryRunAPIClient) AddMembership(username, group string, level internal.Level) error {
	m.Append(fmt.Sprintf("add '%s' to '%s' at level '%s'", username, group, level))
	return nil
}

// ChangeMembership implements the APIClient interface
func (m DryRunAPIClient) ChangeMembership(username, group string, level internal.Level) error {
	m.Append(fmt.Sprintf("change '%s' in '%s' at level '%s'", username, group, level))
	return nil
}

// RemoveMembership implements the APIClient interface
func (m DryRunAPIClient) RemoveMembership(username, group string) error {
	m.Append(fmt.Sprintf("remove '%s' from '%s'", username, group))
	return nil
}

// AddProjectSharing implements the APIClient interface
func (m DryRunAPIClient) AddProjectSharing(project, group string, level internal.Level) error {
	m.Append(fmt.Sprintf("share project '%s' with group '%s' at level '%s'", project, group, level))
	return nil
}

// RemoveProjectSharing implements the APIClient interface
func (m DryRunAPIClient) RemoveProjectSharing(project, group string) error {
	m.Append(fmt.Sprintf("remove project sharing from '%s' with group '%s'", project, group))
	return nil
}
