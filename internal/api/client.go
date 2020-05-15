package api

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
	"gitlab.com/yakshaving.art/hurrdurr/pkg/random"
)

// GitlabAPIClient is a client for proving high level behaviors when talking to
// a GitLab instance
type GitlabAPIClient struct {
	client    *gitlab.Client
	PerPage   int
	ghostUser string

	Querier internal.Querier
}

// GitlabAPIClientArgs gitlab api client
type GitlabAPIClientArgs struct {
	GitlabToken     string
	GitlabBaseURL   string
	GitlabGhostUser string
}

// ErrForbiddenAction is used to indicate that an error is triggered due to the
// user performing an action it's not allowed to
var ErrForbiddenAction = fmt.Errorf("The user is not allowed to run this command")

// NewGitlabAPIClient create a new Gitlab API Client
func NewGitlabAPIClient(args GitlabAPIClientArgs) GitlabAPIClient {
	gitlabClient := gitlab.NewClient(nil, args.GitlabToken)
	if err := gitlabClient.SetBaseURL(args.GitlabBaseURL); err != nil {
		logrus.Fatalf("Could not set base URL '%s' to GitLab Client: '%s'", args.GitlabBaseURL, err)
	}

	return GitlabAPIClient{
		client:    gitlabClient,
		PerPage:   100,
		ghostUser: args.GitlabGhostUser,
	}
}

// CurrentUser returns the user that is used to talk to the API
func (m GitlabAPIClient) CurrentUser() string {
	u, _, err := m.client.Users.CurrentUser()
	if err != nil {
		logrus.Fatalf("Failed to get current user: %s", err)
	}
	return u.Username
}

// AddGroupMembership implements the APIClient interface
func (m GitlabAPIClient) AddGroupMembership(username, group string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.AddGroupMemberOptions{
		UserID:      &userID,
		AccessLevel: &acl,
	}

	_, _, err := m.client.GroupMembers.AddGroupMember(group, opt)
	if err != nil {
		return fmt.Errorf("failed to add user '%s' to group '%s': %s", username, group, err)
	}
	logrus.Printf("[apply] '%s' to '%s' at level '%s'\n", username, group, level)
	return nil
}

// ChangeGroupMembership implements the APIClient interface
func (m GitlabAPIClient) ChangeGroupMembership(username, group string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.EditGroupMemberOptions{
		AccessLevel: &acl,
	}
	_, _, err := m.client.GroupMembers.EditGroupMember(group, userID, opt)
	if err != nil {
		return fmt.Errorf("failed to change user '%s' in group '%s': %s", username, group, err)
	}

	logrus.Printf("[apply] changed '%s' in '%s' at level '%s'\n", username, group, level)
	return nil
}

// RemoveGroupMembership implements the APIClient interface
func (m GitlabAPIClient) RemoveGroupMembership(username, group string) error {
	userID := m.Querier.GetUserID(username)

	_, err := m.client.GroupMembers.RemoveGroupMember(group, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user '%s' from group '%s': %s", username, group, err)
	}
	logrus.Printf("[apply] removed '%s' from '%s'\n", username, group)
	return nil
}

// AddProjectSharing implements the APIClient interface
func (m GitlabAPIClient) AddProjectSharing(project, group string, level internal.Level) error {
	id := m.Querier.GetGroupID(group)
	acl := gitlab.AccessLevelValue(level)

	opt := gitlab.ShareWithGroupOptions{
		GroupID:     &id,
		GroupAccess: &acl,
	}
	_, err := m.client.Projects.ShareProjectWithGroup(project, &opt)
	if err != nil {
		return fmt.Errorf("failed to share project '%s' with group '%s': %s", project, group, err)
	}
	logrus.Printf("[apply] project '%s' shared with '%s' at level '%s'\n", project, group, level)
	return nil
}

// RemoveProjectSharing implements the APIClient interface
func (m GitlabAPIClient) RemoveProjectSharing(project, group string) error {
	id := m.Querier.GetGroupID(group)

	_, err := m.client.Projects.DeleteSharedProjectFromGroup(project, id)
	if err != nil {
		return fmt.Errorf("failed to remove project '%s' sharing with '%s': %s", project, group, err)
	}
	logrus.Printf("[apply] project '%s' is not shared with '%s' anymore\n", project, group)
	return nil
}

// AddProjectMembership implements the APIClient interface
func (m GitlabAPIClient) AddProjectMembership(username, project string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.AddProjectMemberOptions{
		UserID:      &userID,
		AccessLevel: &acl,
	}

	_, _, err := m.client.ProjectMembers.AddProjectMember(project, opt)
	if err != nil {
		return fmt.Errorf("failed to add user '%s' to project '%s': %s", username, project, err)
	}
	logrus.Printf("[apply] added '%s' to '%s' at level '%s'\n", username, project, level)
	return nil
}

// ChangeProjectMembership implements the APIClient interface
func (m GitlabAPIClient) ChangeProjectMembership(username, project string, level internal.Level) error {
	userID := m.Querier.GetUserID(username)
	acl := gitlab.AccessLevelValue(level)

	opt := &gitlab.EditProjectMemberOptions{
		AccessLevel: &acl,
	}
	_, _, err := m.client.ProjectMembers.EditProjectMember(project, userID, opt)
	if err != nil {
		return fmt.Errorf("failed to change user '%s' in project '%s': %s", username, project, err)
	}

	logrus.Printf("[apply] user '%s' changed in '%s' to level '%s'\n", username, project, level)
	return nil
}

// RemoveProjectMembership implements the APIClient interface
func (m GitlabAPIClient) RemoveProjectMembership(username, project string) error {
	userID := m.Querier.GetUserID(username)

	_, err := m.client.ProjectMembers.DeleteProjectMember(project, userID)
	if err != nil {
		return fmt.Errorf("failed to remove user '%s' from project '%s': %s", username, project, err)
	}
	logrus.Printf("[apply] user '%s' removed from '%s'\n", username, project)
	return nil
}

// BlockUser implements the APIClient interface
func (m GitlabAPIClient) BlockUser(username string) error {
	userID := m.Querier.GetUserID(username)

	err := m.client.Users.BlockUser(userID)
	if err != nil {
		return fmt.Errorf("failed to block user '%s': %s", username, err)
	}
	logrus.Printf("[apply] user '%s' is blocked\n", username)

	return nil
}

// UnblockUser implements the APIClient interface
func (m GitlabAPIClient) UnblockUser(username string) error {
	userID := m.Querier.GetUserID(username)

	err := m.client.Users.UnblockUser(userID)
	if err != nil {
		return fmt.Errorf("failed to unblock user '%s': %s", username, err)
	}
	logrus.Printf("[apply] user '%s' is unblocked\n", username)

	return nil
}

// SetAdminUser implements the APIClient interface
func (m GitlabAPIClient) SetAdminUser(username string) error {
	userID := m.Querier.GetUserID(username)
	t := true

	_, _, err := m.client.Users.ModifyUser(userID,
		&gitlab.ModifyUserOptions{
			Admin: &t,
		})
	if err != nil {
		return fmt.Errorf("failed to set user '%s' as admin: %s", username, err)
	}
	logrus.Printf("[apply] user '%s' is admin now\n", username)

	return nil
}

// UnsetAdminUser implements the APIClient interface
func (m GitlabAPIClient) UnsetAdminUser(username string) error {
	userID := m.Querier.GetUserID(username)
	f := false

	_, _, err := m.client.Users.ModifyUser(userID,
		&gitlab.ModifyUserOptions{
			Admin: &f,
		})
	if err != nil {
		return fmt.Errorf("failed to unset user '%s' as admin: %s", username, err)
	}
	logrus.Printf("[apply] user '%s' is not admin anymore\n", username)

	return nil
}

// CreateGroupVariable implements APIClient interface
func (m GitlabAPIClient) CreateGroupVariable(group, key, value string) error {
	_, _, err := m.client.GroupVariables.CreateVariable(group,
		&gitlab.CreateVariableOptions{
			Key:   &key,
			Value: &value,
		})
	if err != nil {
		return fmt.Errorf("failed to create group variable '%s' in group '%s'", key, group)
	}
	logrus.Printf("[apply] variable '%s' in group '%s' was created\n", key, group)
	return nil
}

// UpdateGroupVariable implements APIClient interface
func (m GitlabAPIClient) UpdateGroupVariable(group, key, value string) error {
	_, _, err := m.client.GroupVariables.UpdateVariable(group, key,
		&gitlab.UpdateVariableOptions{
			Value: &value,
		})
	if err != nil {
		return fmt.Errorf("failed to update group variable '%s' in group '%s'", key, group)
	}
	logrus.Printf("[apply] variable '%s' in group '%s' was updated\n", key, group)
	return nil
}

// CreateProjectVariable implements APIClient interface
func (m GitlabAPIClient) CreateProjectVariable(fullpath, key, value string) error {
	_, _, err := m.client.ProjectVariables.CreateVariable(fullpath,
		&gitlab.CreateVariableOptions{
			Key:   &key,
			Value: &value,
		})
	if err != nil {
		return fmt.Errorf("failed to create project variable '%s' in group '%s'", key, fullpath)
	}
	logrus.Printf("[apply] variable '%s' in project '%s' was created\n", key, fullpath)
	return nil
}

// UpdateProjectVariable implements APIClient interface
func (m GitlabAPIClient) UpdateProjectVariable(fullpath, key, value string) error {
	_, _, err := m.client.ProjectVariables.UpdateVariable(fullpath, key,
		&gitlab.UpdateVariableOptions{
			Value: &value,
		})
	if err != nil {
		return fmt.Errorf("failed to update project variable '%s' in group '%s'", key, fullpath)
	}
	logrus.Printf("[apply] variable '%s' in project '%s' was updated\n", key, fullpath)
	return nil
}

// CreateBotUser creates a bot user
func (m GitlabAPIClient) CreateBotUser(username, email string) error {
	p := random.Password(32)
	name := fmt.Sprintf("[BOT] %s", username)
	_, _, err := m.client.Users.CreateUser(&gitlab.CreateUserOptions{
		Username:         &username,
		Password:         &p,
		Name:             &name,
		Email:            &email,
		SkipConfirmation: b(true),
	})
	if err != nil {
		return fmt.Errorf("failed to create bot user '%s': %s", username, err)
	}
	logrus.Printf("[apply] bot user '%s' created", username)
	return nil
}

// UpdateBotEmail implements APIClient interface
func (m GitlabAPIClient) UpdateBotEmail(username, email string) error {
	logrus.Debugf("finding bot user ID '%s' to update email to '%s'", username, email)
	botUserID := m.Querier.GetUserID(username)
	_, _, err := m.client.Users.ModifyUser(
		botUserID,
		&gitlab.ModifyUserOptions{
			Email: &email,
		})
	if err != nil {
		return fmt.Errorf("failed to update bot user '%s' email to '%s': %s", username, email, err)
	}
	logrus.Printf("[apply] bot user '%s' email changed to '%s'", username, email)
	return nil
}

// ########################
// PRIVATE GITLAB API usage
// ########################

func (m GitlabAPIClient) fetchAllUsers(ch chan gitlab.User, errs *errors.Errors) {
	defer close(ch)

	page := 1
	for {
		opt := &gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}
		users, resp, err := m.client.Users.ListUsers(opt)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch users: %s", err))
			break
		}

		for _, user := range users {
			ch <- *user
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
}

func (m GitlabAPIClient) fetchUser(username string) *gitlab.User {
	users, _, err := m.client.Users.ListUsers(&gitlab.ListUsersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 1,
			Page:    1,
		},
		Username: &username,
	})
	if err != nil {
		logrus.Fatalf("failed to fetch user '%s': %s", username, err)
	}

	if len(users) == 0 {
		return nil
	}
	return users[0]
}

func (m GitlabAPIClient) fetchGroup(fullpath string) *gitlab.Group {
	group, _, err := m.client.Groups.GetGroup(fullpath)
	if err != nil {
		logrus.Fatalf("failed to fetch group '%s': %s", fullpath, err)
	}

	return group
}

func (m GitlabAPIClient) fetchGroups(allAvailable bool, ch chan gitlab.Group, errs *errors.Errors) {
	defer close(ch)

	page := 1
	for {
		opt := &gitlab.ListGroupsOptions{
			AllAvailable: &allAvailable,
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}

		groups, resp, err := m.client.Groups.ListGroups(opt)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch groups: %s", err))
			break
		}

		for _, group := range groups {
			ch <- *group
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
}

func (m GitlabAPIClient) fetchGroupMembers(fullpath string) (map[string]internal.Level, error) {
	groupMembers := make(map[string]internal.Level)

	page := 1
	for {
		opt := &gitlab.ListGroupMembersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}

		members, resp, err := m.client.Groups.ListGroupMembers(fullpath, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch group members for %s: %s", fullpath, err)
		}

		for _, member := range members {
			groupMembers[member.Username] = internal.Level(member.AccessLevel)
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
	return groupMembers, nil
}

func (m GitlabAPIClient) fetchGroupVariables(fullpath string) (map[string]string, error) {
	variables := make(map[string]string)

	vars, resp, err := m.client.GroupVariables.ListVariables(fullpath)
	if err != nil {
		if resp.StatusCode == http.StatusForbidden {
			return nil, ErrForbiddenAction
		}
		return nil, fmt.Errorf("failed to list group variables for %s: %s", fullpath, err)
	}

	for _, v := range vars {
		variables[v.Key] = v.Value
	}

	return variables, nil
}

func (m GitlabAPIClient) fetchAllProjects(ch chan gitlab.Project, errs *errors.Errors) {
	defer close(ch)

	page := 1
	for {
		opt := &gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}

		prjs, resp, err := m.client.Projects.ListProjects(opt)
		if err != nil {
			errs.Append(fmt.Errorf("failed to fetch the list of projects: %s", err))
			return
		}

		for _, p := range prjs {
			ch <- *p
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
}

func (m GitlabAPIClient) fetchProjectMembers(fullpath string) (map[string]internal.Level, error) {
	projectMembers := make(map[string]internal.Level)

	page := 1
	for {
		opt := &gitlab.ListProjectMembersOptions{
			ListOptions: gitlab.ListOptions{
				PerPage: m.PerPage,
				Page:    page,
			},
		}

		members, resp, err := m.client.ProjectMembers.ListProjectMembers(fullpath, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch project members for %s: %s", fullpath, err)
		}

		for _, member := range members {
			projectMembers[member.Username] = internal.Level(member.AccessLevel)
		}

		if page == resp.TotalPages {
			break
		}
		page++
	}
	return projectMembers, nil
}

func (m GitlabAPIClient) fetchProjectVariables(fullpath string) (map[string]string, error) {
	projectVariables := make(map[string]string)

	vars, resp, err := m.client.ProjectVariables.ListVariables(fullpath)
	if err != nil {
		if resp.StatusCode == http.StatusForbidden {
			return nil, ErrForbiddenAction
		}
		return nil, fmt.Errorf("failed to list project variables for %s: %s", fullpath, err)
	}
	for _, v := range vars {
		projectVariables[v.Key] = v.Value
	}
	return projectVariables, nil
}

func (m GitlabAPIClient) fetchProject(fullpath string) (*gitlab.Project, error) {
	p, _, err := m.client.Projects.GetProject(fullpath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project '%s': %s", fullpath, err)
	}
	return p, nil
}
