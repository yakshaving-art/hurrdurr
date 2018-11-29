package state

import (
	"fmt"
	"sort"

	"gitlab.com/yakshaving.art/hurrdurr/internal"
	"gitlab.com/yakshaving.art/hurrdurr/internal/errors"
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

	for _, desiredGroup := range desiredGroups {
		desiredMembers := desiredGroup.GetMembers()

		currentGroup, ok := current.Group(desiredGroup.GetFullpath())
		if !ok {
			for desiredName, desiredLevel := range desiredMembers {
				actions = append(actions, addUserMembershipAction{
					Group:    desiredGroup.GetFullpath(),
					Username: desiredName,
					Level:    desiredLevel})
			}
			continue
		}

		currentMembers := currentGroup.GetMembers()

		for desiredName, desiredLevel := range desiredMembers {
			currentLevel, present := currentMembers[desiredName]
			if !present {
				actions = append(actions, addUserMembershipAction{
					Group:    currentGroup.GetFullpath(),
					Username: desiredName,
					Level:    desiredLevel})
				continue
			}

			if currentLevel != desiredLevel {
				actions = append(actions, changeUserLevelAction{
					Group:    currentGroup.GetFullpath(),
					Username: desiredName,
					Level:    desiredLevel})
				continue
			}

			// Do nothing, there's no change
		}

		for current := range currentMembers {
			if _, present := desiredMembers[current]; !present {
				actions = append(actions, removeUserAction{Username: current, Group: currentGroup.GetFullpath()})
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
	return c.ChangeMembership(s.Username, s.Group, int(s.Level))
}

type addUserMembershipAction struct {
	Username string
	Group    string
	Level    internal.Level
}

func (s addUserMembershipAction) Execute(c internal.APIClient) error {
	return c.AddMembership(s.Username, s.Group, int(s.Level))
}

type removeUserAction struct {
	Username string
	Group    string
}

func (r removeUserAction) Execute(c internal.APIClient) error {
	return c.RemoveMembership(r.Username, r.Group)
}
