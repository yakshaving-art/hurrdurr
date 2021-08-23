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

// AddGroupSharing implements the APIClient interface
func (m DryRunAPIClient) AddGroupSharing(group, shared_group string, level internal.Level) error {
	m.Append(fmt.Sprintf("share group '%s' with group '%s' at level '%s'", group, shared_group, level))
	return nil
}

// RemoveGroupSharing implements the APIClient interface
func (m DryRunAPIClient) RemoveGroupSharing(group, shared_group string) error {
	m.Append(fmt.Sprintf("remove group sharing from '%s' with group '%s'", group, shared_group))
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
	m.Append(fmt.Sprintf("change '%s' in '%s' to level '%s'", username, project, level))
	return nil
}

// RemoveProjectMembership implements the APIClient interface
func (m DryRunAPIClient) RemoveProjectMembership(username, project string) error {
	m.Append(fmt.Sprintf("remove '%s' from '%s'", username, project))
	return nil
}

// BlockUser implements the APIClient interface
func (m DryRunAPIClient) BlockUser(username string) error {
	m.Append(fmt.Sprintf("block '%s'", username))
	return nil
}

// UnblockUser implements the APIClient interface
func (m DryRunAPIClient) UnblockUser(username string) error {
	m.Append(fmt.Sprintf("unblock '%s'", username))
	return nil
}

// SetAdminUser implements the APIClient interface
func (m DryRunAPIClient) SetAdminUser(username string) error {
	m.Append(fmt.Sprintf("set '%s' as admin", username))
	return nil
}

// UnsetAdminUser implements the APIClient interface
func (m DryRunAPIClient) UnsetAdminUser(username string) error {
	m.Append(fmt.Sprintf("unset '%s' as admin", username))
	return nil
}

// CreateGroupVariable implements APIClient interface
func (m DryRunAPIClient) CreateGroupVariable(group, key, value string) error {
	m.Append(fmt.Sprintf("create group variable '%s' in '%s'", key, group))
	return nil
}

// UpdateGroupVariable implements APIClient interface
func (m DryRunAPIClient) UpdateGroupVariable(group, key, value string) error {
	m.Append(fmt.Sprintf("update group variable '%s' in '%s'", key, group))
	return nil
}

// CreateProjectVariable implements APIClient interface
func (m DryRunAPIClient) CreateProjectVariable(fullpath, key, value string) error {
	m.Append(fmt.Sprintf("create project variable '%s' in '%s'", key, fullpath))
	return nil
}

// UpdateProjectVariable implements APIClient interface
func (m DryRunAPIClient) UpdateProjectVariable(fullpath, key, value string) error {
	m.Append(fmt.Sprintf("update project variable '%s' in '%s'", key, fullpath))
	return nil
}

// CreateBotUser implements APIClient interface
func (m DryRunAPIClient) CreateBotUser(username, email string) error {
	m.Append(fmt.Sprintf("create bot user '%s' with email '%s", username, email))
	return nil
}

// UpdateBotEmail implements APIClient interface
func (m DryRunAPIClient) UpdateBotEmail(username, desiredEmail string) error {
	m.Append(fmt.Sprintf("update bot '%s' email to '%s'", username, desiredEmail))
	return nil
}
