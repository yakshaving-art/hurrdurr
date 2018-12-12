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
}

type differ struct {
	actions          []internal.Action
	errs             errors.Errors
	current, desired internal.State
}

func (d *differ) Action(a internal.Action) {
	d.actions = append(d.actions, a)
}

func (d *differ) Error(e error) {
	d.errs.Append(e)
}

// Diff returns a set of actions that will turn the current state into the
// desired state
func Diff(current, desired internal.State) ([]internal.Action, error) {
	if current == nil {
		return nil, fmt.Errorf("invalid current state: nil")
	}
	if desired == nil {
		return nil, fmt.Errorf("invalid desired state: nil")
	}

	differ := &differ{
		actions: make([]internal.Action, 0),
		errs:    errors.New(),
		current: current,
		desired: desired,
	}

	differ.diffGroups()
	differ.diffProjects()

	return differ.actions, differ.errs.ErrorOrNil()
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
			logrus.Debugf("  Diffing desired group %s members because the current group is present", desiredGroup.GetFullpath())

			for _, m := range sortedMembers(desiredMembers) {
				desiredName := m.name
				desiredLevel := m.level

				currentLevel, currentMemberPresent := currentMembers[desiredName]
				if !currentMemberPresent {
					logrus.Debugf("  Adding %s to group %s at level %s", desiredName, desiredGroup.GetFullpath(), desiredLevel)
					d.Action(addUserMembershipAction{
						Group:    desiredGroup.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})
				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing %s in group %s to level %s", desiredName, desiredGroup.GetFullpath(), desiredLevel)
					d.Action(changeUserLevelAction{
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
						d.current, currentGroup.GetFullpath())
					d.Action(removeUserAction{Username: currentMember, Group: currentGroup.GetFullpath()})
				}
			}
		} else { // !currentGroupPresent
			logrus.Debugf("  Appending desired group %s members because the current group is not present", desiredGroup.GetFullpath())
			for desiredName, desiredLevel := range desiredMembers {
				logrus.Debugf("  Adding %s to group %s at level %s", desiredName, desiredGroup.GetFullpath(), desiredLevel)
				d.Action(addUserMembershipAction{
					Group:    desiredGroup.GetFullpath(),
					Username: desiredName,
					Level:    desiredLevel})
			}
		}

	}

}

func (d *differ) diffProjects() {
	for _, desiredProject := range d.desired.Projects() {
		currentProject, currentProjectPresent := d.current.Project(desiredProject.GetFullpath())

		logrus.Debugf("Processing desired project state: %#v from current state: %#v", desiredProject, currentProject)

		if currentProjectPresent {
			logrus.Debugf("  Diffing project %s because current project is present", currentProject.GetFullpath())
			for desiredGroup, desiredLevel := range desiredProject.GetSharedGroups() {
				currentLevel, currentLevelPresent := currentProject.GetSharedGroups()[desiredGroup]
				if !currentLevelPresent {
					logrus.Debugf("  Adding group %s sharing because current level is not currently present", desiredGroup)
					d.Action(shareProjectWithGroup{
						Project: desiredProject.GetFullpath(),
						Group:   desiredGroup,
						Level:   desiredLevel,
					})
				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing group %s sharing as %s because current level is %s", desiredGroup, desiredLevel, currentLevel)

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
					logrus.Debugf("  Adding project %s membership for %s as %s", desiredProject.GetFullpath(), desiredName, desiredLevel)
					d.Action(addProjectMembership{
						Project:  desiredProject.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})

				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing project %s membership for %s to %s", desiredProject.GetFullpath(), desiredName, desiredLevel)
					d.Action(changeProjectMembership{
						Project:  desiredProject.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})
				}

			}

		} else { // currentProject not present
			logrus.Debugf("  Appending project %s because current state is not present", desiredProject.GetFullpath())

			for desiredGroup, desiredLevel := range desiredProject.GetSharedGroups() {
				logrus.Debugf("  Adding group %s sharing with %s because project is not currently present", desiredProject.GetFullpath(), desiredGroup)
				d.Action(shareProjectWithGroup{
					Project: desiredProject.GetFullpath(),
					Group:   desiredGroup,
					Level:   desiredLevel,
				})
			}

			desiredMembers := desiredProject.GetMembers()
			for desiredName, desiredLevel := range desiredMembers {
				logrus.Debugf("  Adding project %s membership for %s as %s", desiredProject.GetFullpath(), desiredName, desiredLevel)
				d.Action(addProjectMembership{
					Username: desiredName,
					Project:  desiredProject.GetFullpath(),
					Level:    desiredLevel,
				})
			}
		}
	}

	// Compare current project state with desired to remove things
	for _, currentProject := range d.current.Projects() {
		desiredProject, desiredProjectPresent := d.desired.Project(currentProject.GetFullpath())
		if !desiredProjectPresent {
			logrus.Debugf("Skipping current project '%s' because it's not managed in desired state", currentProject.GetFullpath())
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

type changeUserLevelAction struct {
	Username string
	Group    string
	Level    internal.Level
}

func (s changeUserLevelAction) Execute(c internal.APIClient) error {
	return c.ChangeGroupMembership(s.Username, s.Group, s.Level)
}

type addUserMembershipAction struct {
	Username string
	Group    string
	Level    internal.Level
}

func (s addUserMembershipAction) Execute(c internal.APIClient) error {
	return c.AddGroupMembership(s.Username, s.Group, s.Level)
}

type removeUserAction struct {
	Username string
	Group    string
}

func (r removeUserAction) Execute(c internal.APIClient) error {
	return c.RemoveGroupMembership(r.Username, r.Group)
}

type shareProjectWithGroup struct {
	Project string
	Group   string
	Level   internal.Level
}

func (r shareProjectWithGroup) Execute(c internal.APIClient) error {
	return c.AddProjectSharing(r.Project, r.Group, r.Level)
}

type removeProjectGroupSharing struct {
	Project string
	Group   string
}

func (r removeProjectGroupSharing) Execute(c internal.APIClient) error {
	return c.RemoveProjectSharing(r.Project, r.Group)
}

type addProjectMembership struct {
	Project  string
	Username string
	Level    internal.Level
}

func (r addProjectMembership) Execute(c internal.APIClient) error {
	return c.AddProjectMembership(r.Username, r.Project, r.Level)
}

type changeProjectMembership struct {
	Project  string
	Username string
	Level    internal.Level
}

func (r changeProjectMembership) Execute(c internal.APIClient) error {
	return c.ChangeProjectMembership(r.Username, r.Project, r.Level)
}

type removeProjectMembership struct {
	Project  string
	Username string
}

func (r removeProjectMembership) Execute(c internal.APIClient) error {
	return c.RemoveProjectMembership(r.Username, r.Project)
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
