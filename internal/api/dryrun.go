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

// AddGroupMembership implements the APIClient interface
func (m DryRunAPIClient) AddGroupMembership(username, group string, level internal.Level) error {
	m.Append(fmt.Sprintf("add '%s' to '%s' at level '%s'", username, group, level))
	return nil
}

// ChangeGroupMembership implements the APIClient interface
func (m DryRunAPIClient) ChangeGroupMembership(username, group string, level internal.Level) error {
	m.Append(fmt.Sprintf("change '%s' in '%s' at level '%s'", username, group, level))
	return nil
}

// RemoveGroupMembership implements the APIClient interface
func (m DryRunAPIClient) RemoveGroupMembership(username, group string) error {
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

// AddProjectMembership implements the APIClient interface
func (m DryRunAPIClient) AddProjectMembership(username, project string, level internal.Level) error {
	m.Append(fmt.Sprintf("add '%s' to '%s' at level '%s'", username, project, level))
	return nil
}

// ChangeProjectMembership implements the APIClient interface
func (m DryRunAPIClient) ChangeProjectMembership(username, project string, level internal.Level) error {
	m.Append(fmt.Sprintf("change '%s' in '%s' at level '%s'", username, project, level))
	return nil
}

// RemoveProjectMembership implements the APIClient interface
func (m DryRunAPIClient) RemoveProjectMembership(username, project string) error {
	m.Append(fmt.Sprintf("remove '%s' from '%s'", username, project))
	return nil
}
