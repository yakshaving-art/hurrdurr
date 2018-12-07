package state

import (
	"fmt"
	"sort"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"

	"github.com/sirupsen/logrus"
)

// Diff returns a set of actions that will turn the current state into the
// desired state
func Diff(current, desired internal.State) ([]internal.Action, error) {
	if current == nil {
		return nil, fmt.Errorf("invalid current state: nil")
	}
	if desired == nil {
		return nil, fmt.Errorf("invalid desired state: nil")
	}

	actions := make([]internal.Action, 0)
	errs := errors.New()

	desiredGroups := desired.Groups()
	sort.Slice(desiredGroups, func(i, j int) bool {
		return desiredGroups[i].GetFullpath() < desiredGroups[j].GetFullpath()
	})

	logrus.Debugf("Comparing current %#v with desired %#v", current, desired)

	for _, desiredGroup := range desiredGroups {
		logrus.Debugf("Processing desired group %s", desiredGroup.GetFullpath())

		currentGroup, currentGroupPresent := current.Group(desiredGroup.GetFullpath())
		desiredMembers := desiredGroup.GetMembers()

		if currentGroupPresent {
			currentMembers := currentGroup.GetMembers()
			logrus.Debugf("  Diffing desired group %s members because the current group is present", desiredGroup.GetFullpath())

			for desiredName, desiredLevel := range desiredMembers {
				currentLevel, currentMemberPresent := currentMembers[desiredName]
				if !currentMemberPresent {
					logrus.Debugf("  Adding %s to group %s at level %s", desiredName, desiredGroup.GetFullpath(), desiredLevel)
					actions = append(actions, addUserMembershipAction{
						Group:    desiredGroup.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})
				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing %s in group %s to level %s", desiredName, desiredGroup.GetFullpath(), desiredLevel)
					actions = append(actions, changeUserLevelAction{
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
						current, currentGroup.GetFullpath())
					actions = append(actions, removeUserAction{Username: currentMember, Group: currentGroup.GetFullpath()})
				}
			}
		} else { // !currentGroupPresent
			logrus.Debugf("  Appending desired group %s members because the current group is not present", desiredGroup.GetFullpath())
			for desiredName, desiredLevel := range desiredMembers {
				logrus.Debugf("  Adding %s to group %s at level %s", desiredName, desiredGroup.GetFullpath(), desiredLevel)
				actions = append(actions, addUserMembershipAction{
					Group:    desiredGroup.GetFullpath(),
					Username: desiredName,
					Level:    desiredLevel})
			}
		}

	}

	for _, desiredProject := range desired.Projects() {
		currentProject, currentProjectPresent := current.Project(desiredProject.GetFullpath())

		logrus.Debugf("Processing desired project state: %#v from current state: %#v", desiredProject, currentProject)

		if currentProjectPresent {
			logrus.Debugf("  Diffing project %s because current project is present", currentProject.GetFullpath())
			for desiredGroup, desiredLevel := range desiredProject.GetSharedGroups() {
				currentLevel, currentLevelPresent := currentProject.GetSharedGroups()[desiredGroup]
				if !currentLevelPresent {
					logrus.Debugf("  Adding group %s sharing because current level is not currently present", desiredGroup)
					actions = append(actions, shareProjectWithGroup{
						Project: desiredProject.GetFullpath(),
						Group:   desiredGroup,
						Level:   desiredLevel,
					})
				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing group %s sharing as %s because current level is %s", desiredGroup, desiredLevel, currentLevel)

					actions = append(actions, removeProjectGroupSharing{
						Project: desiredProject.GetFullpath(),
						Group:   desiredGroup,
					})

					actions = append(actions, shareProjectWithGroup{
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
					actions = append(actions, addProjectMembership{
						Project:  desiredProject.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})

				} else if currentLevel != desiredLevel {
					logrus.Debugf("  Changing project %s membership for %s to %s", desiredProject.GetFullpath(), desiredName, desiredLevel)
					actions = append(actions, changeProjectMembership{
						Project:  desiredProject.GetFullpath(),
						Username: desiredName,
						Level:    desiredLevel})
				}

			}

		} else { // currentProject not present
			logrus.Debugf("  Appending project %s because current state is not present", desiredProject.GetFullpath())

			for desiredGroup, desiredLevel := range desiredProject.GetSharedGroups() {
				logrus.Debugf("  Adding group %s sharing with %s because project is not currently present", desiredProject.GetFullpath(), desiredGroup)
				actions = append(actions, shareProjectWithGroup{
					Project: desiredProject.GetFullpath(),
					Group:   desiredGroup,
					Level:   desiredLevel,
				})
			}

			desiredMembers := desiredProject.GetMembers()
			for desiredName, desiredLevel := range desiredMembers {
				logrus.Debugf("  Adding project %s membership for %s as %s", desiredProject.GetFullpath(), desiredName, desiredLevel)
				actions = append(actions, addProjectMembership{
					Username: desiredName,
					Project:  desiredProject.GetFullpath(),
					Level:    desiredLevel,
				})
			}
		}
	}

	// Compare current project state with desired to remove things
	for _, currentProject := range current.Projects() {
		desiredProject, desiredProjectPresent := desired.Project(currentProject.GetFullpath())
		for group := range currentProject.GetSharedGroups() {
			if !desiredProjectPresent {
				logrus.Debugf("  Removing project %s sharing with group %s because project is not in the desired state",
					currentProject.GetFullpath(), group)

				actions = append(actions, removeProjectGroupSharing{
					Project: currentProject.GetFullpath(),
					Group:   group,
				})
			} else {
				_, groupPresent := desiredProject.GetSharedGroups()[group]
				if !groupPresent {
					logrus.Debugf("  Removing project %s sharing with group %s because group is not in the desired state",
						currentProject.GetFullpath(), group)

					actions = append(actions, removeProjectGroupSharing{
						Project: currentProject.GetFullpath(),
						Group:   group,
					})

				}
			}
		}

		for member := range currentProject.GetMembers() {
			if desiredProjectPresent {
				_, memberPresent := desiredProject.GetMembers()[member]
				if !memberPresent {
					logrus.Debugf("  Removing project %s membership for %s because member is not in the desired state",
						currentProject.GetFullpath(), member)

					actions = append(actions, removeProjectMembership{
						Project:  currentProject.GetFullpath(),
						Username: member,
					})
				}
			} else {
				logrus.Debugf("  Removing project %s membership for %s because project is not in the desired state",
					currentProject.GetFullpath(), member)

				actions = append(actions, removeProjectMembership{
					Project:  currentProject.GetFullpath(),
					Username: member,
				})

			}
		}
	}

	return actions, errs.ErrorOrNil()
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
