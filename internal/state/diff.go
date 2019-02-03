package state

import (
	"fmt"
	"sort"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"

	"github.com/sirupsen/logrus"
)

// DiffArgs represents the different arguments used to configure the diffing process
type DiffArgs struct {
	DiffGroups   bool
	DiffProjects bool
	DiffUsers    bool

	Yolo bool
}

type differ struct {
	actions          map[internal.ActionPriority][]internal.Action
	errs             errors.Errors
	current, desired internal.State

	yolo bool
}

func (d *differ) Action(a internal.Action) {
	actions, ok := d.actions[a.Priority()]
	if !ok {
		actions = make([]internal.Action, 0)
	}
	actions = append(actions, a)
	d.actions[a.Priority()] = actions
}

func (d differ) prioritizedActions() []internal.Action {
	pactions := make([]internal.Action, 0)
	for _, priority := range []internal.ActionPriority{
		internal.UnblockUser,
		internal.ManageAdminUser,
		internal.ManageGroup,
		internal.ManageProject,
		internal.BlockUser} {
		if actions, ok := d.actions[priority]; ok {
			for _, a := range actions {
				pactions = append(pactions, a)
			}
		}
	}

	return pactions
}

func (d *differ) Error(e error) {
	d.errs.Append(e)
}

// Diff returns a set of actions that will turn the current state into the
// desired state
func Diff(current, desired internal.State, args DiffArgs) ([]internal.Action, error) {
	if current == nil {
		return nil, fmt.Errorf("invalid current state: nil")
	}
	if desired == nil {
		return nil, fmt.Errorf("invalid desired state: nil")
	}

	differ := &differ{
		actions: make(map[internal.ActionPriority][]internal.Action, 0),
		errs:    errors.New(),
		current: current,
		desired: desired,
		yolo:    args.Yolo,
	}

	if args.DiffGroups {
		differ.diffGroups()
	}
	if args.DiffProjects {
		differ.diffProjects()
	}
	if args.DiffUsers {
		differ.diffUsers()
	}

	return differ.prioritizedActions(), differ.errs.ErrorOrNil()
}

func (d *differ) diffGroups() {
	desiredGroups := d.desired.Groups()
	sort.Slice(desiredGroups, func(i, j int) bool {
		return desiredGroups[i].GetFullpath() < desiredGroups[j].GetFullpath()
	})

	for _, desiredGroup := range desiredGroups {
		logrus.Debugf("Processing desired group %s", desiredGroup.GetFullpath())

		currentGroup, currentGroupPresent := d.current.Group(desiredGroup.GetFullpath())
		desiredMembers := desiredGroup.GetMembers()

		if currentGroupPresent {
			currentMembers := currentGroup.GetMembers()
			logrus.Debugf("  Diffing desired group %s members because the current group is present",
				desiredGroup.GetFullpath())

			for _, m := range sortedMembers(desiredMembers) {
				desiredName := m.name
				desiredLevel := m.level

				currentLevel, currentMemberPresent := currentMembers[desiredName]
				if !currentMemberPresent {
					logrus.Debugf("  Adding %s to group %s at level %s", desiredName, desiredGroup.GetFullpath(),
						desiredLevel)
					d.Action(addGroupMembership{
						Group:    desiredGroup.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})
				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing %s in group %s to level %s", desiredName, desiredGroup.GetFullpath(),
						desiredLevel)
					d.Action(changeGroupMembership{
						Group:    desiredGroup.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})
				} else {
					// Do nothing, there's no change
				}
			}

			logrus.Debugf("  Processing current group %s members not in desired state", desiredGroup.GetFullpath())
			for currentMember := range currentMembers {
				if _, desiredMemberPresent := desiredMembers[currentMember]; !desiredMemberPresent {
					logrus.Debugf("  Removing %s from group %s because it's not present in the desired state",
						currentMember, currentGroup.GetFullpath())
					d.Action(removeGroupMembership{Username: currentMember, Group: currentGroup.GetFullpath()})
				}
			}

			logrus.Debugf("  Processing group %s secret vars", desiredGroup.GetFullpath())
			for k, v := range desiredGroup.GetVariables() {
				if !currentGroup.HasVariable(k) {
					d.Action(createGroupVariable{
						Group: desiredGroup.GetFullpath(),
						Key:   k,
						Value: v,
					})
					continue
				}
				if !currentGroup.VariableEquals(k, v) {

					if d.yolo {
						d.Action(updateGroupVariable{
							Group: desiredGroup.GetFullpath(),
							Key:   k,
							Value: v,
						})
					} else {
						d.Error(fmt.Errorf("variable %s in group %s is not as expected", k,
							desiredGroup.GetFullpath()))
					}
				}
			}

		} else { // !currentGroupPresent
			logrus.Debugf("  Appending desired group %s members because the current group is not present",
				desiredGroup.GetFullpath())
			for desiredName, desiredLevel := range desiredMembers {
				logrus.Debugf("  Adding %s to group %s at level %s", desiredName, desiredGroup.GetFullpath(),
					desiredLevel)
				d.Action(addGroupMembership{
					Group:    desiredGroup.GetFullpath(),
					Username: desiredName,
					Level:    desiredLevel})
			}
			for k, v := range desiredGroup.GetVariables() {
				d.Action(createGroupVariable{
					Group: desiredGroup.GetFullpath(),
					Key:   k,
					Value: v,
				})
			}
		}
	}
}

func (d *differ) diffProjects() {
	for _, desiredProject := range d.desired.Projects() {
		currentProject, currentProjectPresent := d.current.Project(desiredProject.GetFullpath())

		logrus.Debugf("Processing desired project state: %#v from current state: %#v",
			desiredProject, currentProject)

		if currentProjectPresent {
			logrus.Debugf("  Diffing project %s because current project is present", currentProject.GetFullpath())
			for desiredGroup, desiredLevel := range desiredProject.GetSharedGroups() {
				currentLevel, currentLevelPresent := currentProject.GetSharedGroups()[desiredGroup]
				if !currentLevelPresent {
					logrus.Debugf("  Adding group %s sharing because current level is not currently present",
						desiredGroup)
					d.Action(shareProjectWithGroup{
						Project: desiredProject.GetFullpath(),
						Group:   desiredGroup,
						Level:   desiredLevel,
					})
				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing group %s sharing as %s because current level is %s",
						desiredGroup, desiredLevel, currentLevel)

					d.Action(removeProjectGroupSharing{
						Project: desiredProject.GetFullpath(),
						Group:   desiredGroup,
					})

					d.Action(shareProjectWithGroup{
						Project: desiredProject.GetFullpath(),
						Group:   desiredGroup,
						Level:   desiredLevel,
					})
				} else {
					logrus.Debugf("  Keeping group %s sharing as is", desiredGroup)
				}
			}

			logrus.Debugf("Comparing project members for %s with both states present", desiredProject.GetFullpath())

			desiredMembers := desiredProject.GetMembers()
			currentMembers := currentProject.GetMembers()

			for desiredName, desiredLevel := range desiredMembers {

				currentLevel, currentMemberPresent := currentMembers[desiredName]
				if !currentMemberPresent {
					logrus.Debugf("  Adding project %s membership for %s as %s", desiredProject.GetFullpath(),
						desiredName, desiredLevel)
					d.Action(addProjectMembership{
						Project:  desiredProject.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})

				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing project %s membership for %s to %s", desiredProject.GetFullpath(),
						desiredName, desiredLevel)
					d.Action(changeProjectMembership{
						Project:  desiredProject.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})
				}

				logrus.Debugf("  Processing project %s secret vars", desiredProject.GetFullpath())
				for k, v := range desiredProject.GetVariables() {
					if !currentProject.HasVariable(k) {
						d.Action(createProjectVariable{
							Project: desiredProject.GetFullpath(),
							Key:     k,
							Value:   v,
						})
						continue
					}
					if !currentProject.VariableEquals(k, v) {
						if d.yolo {
							d.Action(updateProjectVariable{
								Project: desiredProject.GetFullpath(),
								Key:     k,
								Value:   v,
							})
						} else {
							d.Error(fmt.Errorf("variable %s in project %s is not as expected", k,
								desiredProject.GetFullpath()))
						}
					}
				}
			}

		} else { // currentProject not present
			logrus.Debugf("  Appending project %s because current state is not present", desiredProject.GetFullpath())

			for desiredGroup, desiredLevel := range desiredProject.GetSharedGroups() {
				logrus.Debugf("  Adding group %s sharing with %s because project is not currently present",
					desiredProject.GetFullpath(), desiredGroup)
				d.Action(shareProjectWithGroup{
					Project: desiredProject.GetFullpath(),
					Group:   desiredGroup,
					Level:   desiredLevel,
				})
			}

			desiredMembers := desiredProject.GetMembers()
			for desiredName, desiredLevel := range desiredMembers {
				logrus.Debugf("  Adding project %s membership for %s as %s", desiredProject.GetFullpath(),
					desiredName, desiredLevel)
				d.Action(addProjectMembership{
					Username: desiredName,
					Project:  desiredProject.GetFullpath(),
					Level:    desiredLevel,
				})
			}

			for k, v := range desiredProject.GetVariables() {
				d.Action(createProjectVariable{
					Project: desiredProject.GetFullpath(),
					Key:     k,
					Value:   v,
				})
			}
		}
	}

	// Compare current project state with desired to remove things
	for _, currentProject := range d.current.Projects() {
		desiredProject, desiredProjectPresent := d.desired.Project(currentProject.GetFullpath())
		if !desiredProjectPresent {
			logrus.Debugf("Skipping current project '%s' because it's not managed in desired state",
				currentProject.GetFullpath())
			continue
		}

		for group := range currentProject.GetSharedGroups() {
			if _, desiredGroupPresent := desiredProject.GetGroupLevel(group); !desiredGroupPresent {
				logrus.Debugf("  Removing project %s sharing with group %s because project is not in the desired state",
					currentProject.GetFullpath(), group)

				d.Action(removeProjectGroupSharing{
					Project: currentProject.GetFullpath(),
					Group:   group,
				})
			}
		}

		for member := range currentProject.GetMembers() {
			_, memberPresent := desiredProject.GetMembers()[member]
			if !memberPresent {
				logrus.Debugf("  Removing project %s membership for %s because member is not in the desired state",
					currentProject.GetFullpath(), member)

				d.Action(removeProjectMembership{
					Project:  currentProject.GetFullpath(),
					Username: member,
				})
			}
		}
	}

}

func (d *differ) diffUsers() {
	for _, a := range d.desired.Admins() {
		if !d.current.IsAdmin(a) {
			d.Action(setAdminUser{
				Username: a,
			})
		}
	}

	for _, a := range d.current.Admins() {
		if !d.desired.IsAdmin(a) {
			if d.desired.CurrentUser() == a {
				d.Error(fmt.Errorf("can't unset current user '%s' as admin as it would be shooting myself in the foot", a))
				continue
			}
			d.Action(unsetAdminUser{
				Username: a,
			})
		}
	}

	for _, a := range d.current.Blocked() {
		if !d.desired.IsBlocked(a) {
			d.Action(unblockUser{
				Username: a,
			})
		}
	}

	for _, a := range d.desired.Blocked() {
		if d.desired.CurrentUser() == a {
			d.Error(fmt.Errorf("can't block current user '%s' as it would be shooting myself in the foot", a))
			continue
		}
		if !d.current.IsBlocked(a) {
			d.Action(blockUser{
				Username: a,
			})
		}
	}
}

type changeGroupMembership struct {
	Username string
	Group    string
	Level    internal.Level
}

func (s changeGroupMembership) Execute(c internal.APIClient) error {
	return c.ChangeGroupMembership(s.Username, s.Group, s.Level)
}

func (changeGroupMembership) Priority() internal.ActionPriority {
	return internal.ManageGroup
}

type addGroupMembership struct {
	Username string
	Group    string
	Level    internal.Level
}

func (s addGroupMembership) Execute(c internal.APIClient) error {
	return c.AddGroupMembership(s.Username, s.Group, s.Level)
}

func (addGroupMembership) Priority() internal.ActionPriority {
	return internal.ManageGroup
}

type removeGroupMembership struct {
	Username string
	Group    string
}

func (r removeGroupMembership) Execute(c internal.APIClient) error {
	return c.RemoveGroupMembership(r.Username, r.Group)
}

func (removeGroupMembership) Priority() internal.ActionPriority {
	return internal.ManageGroup
}

type createGroupVariable struct {
	Group string
	Key   string
	Value string
}

func (p createGroupVariable) Execute(c internal.APIClient) error {
	return c.CreateGroupVariable(p.Group, p.Key, p.Value)
}

func (createGroupVariable) Priority() internal.ActionPriority {
	return internal.ManageGroup
}

type updateGroupVariable struct {
	Group string
	Key   string
	Value string
}

func (p updateGroupVariable) Execute(c internal.APIClient) error {
	return c.UpdateGroupVariable(p.Group, p.Key, p.Value)
}

func (updateGroupVariable) Priority() internal.ActionPriority {
	return internal.ManageGroup
}

type shareProjectWithGroup struct {
	Project string
	Group   string
	Level   internal.Level
}

func (r shareProjectWithGroup) Execute(c internal.APIClient) error {
	return c.AddProjectSharing(r.Project, r.Group, r.Level)
}

func (shareProjectWithGroup) Priority() internal.ActionPriority {
	return internal.ManageProject
}

type removeProjectGroupSharing struct {
	Project string
	Group   string
}

func (r removeProjectGroupSharing) Execute(c internal.APIClient) error {
	return c.RemoveProjectSharing(r.Project, r.Group)
}

func (removeProjectGroupSharing) Priority() internal.ActionPriority {
	return internal.ManageProject
}

type addProjectMembership struct {
	Project  string
	Username string
	Level    internal.Level
}

func (r addProjectMembership) Execute(c internal.APIClient) error {
	return c.AddProjectMembership(r.Username, r.Project, r.Level)
}

func (addProjectMembership) Priority() internal.ActionPriority {
	return internal.ManageProject
}

type changeProjectMembership struct {
	Project  string
	Username string
	Level    internal.Level
}

func (r changeProjectMembership) Execute(c internal.APIClient) error {
	return c.ChangeProjectMembership(r.Username, r.Project, r.Level)
}

func (changeProjectMembership) Priority() internal.ActionPriority {
	return internal.ManageProject
}

type removeProjectMembership struct {
	Project  string
	Username string
}

func (r removeProjectMembership) Execute(c internal.APIClient) error {
	return c.RemoveProjectMembership(r.Username, r.Project)
}

func (removeProjectMembership) Priority() internal.ActionPriority {
	return internal.ManageProject
}

type createProjectVariable struct {
	Project string
	Key     string
	Value   string
}

func (p createProjectVariable) Execute(c internal.APIClient) error {
	return c.CreateProjectVariable(p.Project, p.Key, p.Value)
}

func (createProjectVariable) Priority() internal.ActionPriority {
	return internal.ManageProject
}

type updateProjectVariable struct {
	Project string
	Key     string
	Value   string
}

func (p updateProjectVariable) Execute(c internal.APIClient) error {
	return c.UpdateProjectVariable(p.Project, p.Key, p.Value)
}

func (updateProjectVariable) Priority() internal.ActionPriority {
	return internal.ManageProject
}

type setAdminUser struct {
	Username string
}

func (r setAdminUser) Execute(c internal.APIClient) error {
	return c.SetAdminUser(r.Username)
}

func (setAdminUser) Priority() internal.ActionPriority {
	return internal.ManageAdminUser
}

type unsetAdminUser struct {
	Username string
}

func (r unsetAdminUser) Execute(c internal.APIClient) error {
	return c.UnsetAdminUser(r.Username)
}

func (unsetAdminUser) Priority() internal.ActionPriority {
	return internal.ManageAdminUser
}

type blockUser struct {
	Username string
}

func (r blockUser) Execute(c internal.APIClient) error {
	return c.BlockUser(r.Username)
}

func (blockUser) Priority() internal.ActionPriority {
	return internal.BlockUser
}

type unblockUser struct {
	Username string
}

func (r unblockUser) Execute(c internal.APIClient) error {
	return c.UnblockUser(r.Username)
}

func (unblockUser) Priority() internal.ActionPriority {
	return internal.UnblockUser
}

type member struct {
	name  string
	level internal.Level
}

func sortedMembers(members map[string]internal.Level) []member {
	sorted := make([]member, 0)
	for name, level := range members {
		m := member{
			name:  name,
			level: level,
		}

		if m.level == internal.Owner {
			sorted = append([]member{m}, sorted...)
		} else {
			sorted = append(sorted, m)
		}
	}
	return sorted
}
