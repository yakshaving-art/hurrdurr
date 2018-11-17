package main

import (
	"fmt"
	"log"

	"os"
	"strings"

	gitlab "github.com/xanzy/go-gitlab"
)

func readFromEnv(name string) string {
	val, ok := os.LookupEnv(name)
	if !ok {
		log.Fatalf("Environment variable '%s' is required!", name)
	}
	return val
}

func applyBaseURL(baseurl string) {
	if !strings.HasPrefix(baseurl, "https://") {
		log.Fatalf("Validate error: base_url should use https:// scheme")
	}
	if !strings.HasSuffix(baseurl, "/api/v4/") {
		log.Fatalf("Validate error: base_url should end with '/api/v4/'")
	}

	if err := gitlabClient.SetBaseURL(baseurl); err != nil {
		log.Fatalf("Error setting base url: '%+v'", err)
	}
}

// getAllUsers returns array of all users from gitlab instance
func getAllUsers() []gitlab.User {
	var result []gitlab.User

	opt := &gitlab.ListUsersOptions{
		Active: func() *bool { b := true; return &b }(), // maybe there's a better way to init *bool here?
		ListOptions: gitlab.ListOptions{
			PerPage: 50,
			Page:    1,
		},
	}

	users, resp, err := gitlabClient.Users.ListUsers(opt)
	if err != nil {
		log.Fatalf("Error getting users: %v", err)
	}
	for _, user := range users {
		//if isRegularUser(user) { TODO user filtering
		result = append(result, *user)
		//}
	}
	// handle pagination
	for page := 2; page <= resp.TotalPages; page++ {
		opt.ListOptions.Page = page
		users, _, err = gitlabClient.Users.ListUsers(opt)
		if err != nil {
			log.Fatalf("Error getting users: %v", err)
		}
		for _, user := range users {
			//if isRegularUser(user) { TODO
			result = append(result, *user)
			//}
		}
	}
	return result
}

// getAllgroups returns array of all groups from gitlab instance
func getAllGroups() []gitlab.Group {
	var result []gitlab.Group

	opt := &gitlab.ListGroupsOptions{
		AllAvailable: func() *bool { b := true; return &b }(), // maybe there's a better way to init *bool here?
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	groups, resp, err := gitlabClient.Groups.ListGroups(opt)
	if err != nil {
		log.Fatalf("Error getting groups: %v", err)
	}
	for _, group := range groups {
		// TODO: filtering?
		result = append(result, *group)
	}
	// handle pagination
	for page := 2; page <= resp.TotalPages; page++ {
		opt.ListOptions.Page = page
		groups, _, err = gitlabClient.Groups.ListGroups(opt)
		if err != nil {
			log.Fatalf("Error getting users: %v", err)
		}
		for _, group := range groups {
			// TODO: filtering
			result = append(result, *group)
		}
	}
	return result
}

// gitlabGetGroupMembers gets list of all gitlab.User members for a specific group ID
func gitlabGetGroupMembers(groupPath string) []gitlab.GroupMember {
	var result []gitlab.GroupMember

	opt := &gitlab.ListGroupMembersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	members, resp, err := gitlabClient.Groups.ListGroupMembers(groupPath, opt)
	if err != nil {
		log.Fatalf("Error getting members for groupPath '%s': %+v\n", groupPath, err)
	}
	for _, member := range members {
		result = append(result, *member)
	}
	// handle pagination
	for page := 2; page <= resp.TotalPages; page++ {
		opt.ListOptions.Page = page
		members, _, err := gitlabClient.Groups.ListGroupMembers(groupPath, opt)
		if err != nil {
			log.Fatalf("Error getting members for groupPath '%s': %+v\n", groupPath, err)
		}
		for _, member := range members {
			result = append(result, *member)
		}
	}

	return result
}

func removeMemberFromGroup(username string, groupPath string) {
	// TODO: pass this from caller instead
	var userid int
	for _, u := range allUsers {
		if u.Username == username {
			userid = u.ID
			break
		}
	}
	if userid == 0 {
		log.Fatalf("Something very wrong! got ID zero for user '%s'!", username)
	}

	_, err := gitlabClient.GroupMembers.RemoveGroupMember(groupPath, userid)
	if err != nil {
		log.Fatalf("removing member '%s' from groupPath '%s': %+v\n", username, groupPath, err)
	}
	fmt.Printf("\tRemoved user '%s' from group '%s'!\n", username, groupPath)
}

func updateMembersACL(username string, groupPath string, acl int) {
	// TODO: pass this from caller instead
	var userid int
	for _, u := range allUsers {
		if u.Username == username {
			userid = u.ID
			break
		}
	}
	if userid == 0 {
		log.Fatalf("Something very wrong! got ID zero for user '%s'!", username)
	}

	aclv := gitlab.AccessLevelValue(acl)

	opt := &gitlab.EditGroupMemberOptions{
		AccessLevel: &aclv,
	}

	_, _, err := gitlabClient.GroupMembers.EditGroupMember(groupPath, userid, opt)
	if err != nil {
		log.Fatalf("Error updating member '%s' in group '%s' with acl '%d': %+v\n", username, groupPath, acl, err)
	}
	fmt.Printf("\tUpdated user '%s' from group '%s' with acl '%d'\n", username, groupPath, acl)
}

func addMemberToGroup(username string, groupPath string, acl int) {
	// TODO: pass this from caller instead
	var userid int
	for _, u := range allUsers {
		if u.Username == username {
			userid = u.ID
			break
		}
	}
	if userid == 0 {
		log.Fatalf("Something very wrong! got ID zero for user '%s'!", username)
	}

	aclv := gitlab.AccessLevelValue(acl)

	opt := &gitlab.AddGroupMemberOptions{
		UserID:      &userid,
		AccessLevel: &aclv,
	}

	_, _, err := gitlabClient.GroupMembers.AddGroupMember(groupPath, opt)
	if err != nil {
		log.Fatalf("Error adding member '%s' to groupPath '%s': %+v\n", username, groupPath, err)
	}
	fmt.Printf("\tAdded user '%s' to group '%s' with ACL '%d'!\n", username, groupPath, acl)
}

// isRegularUser returns true if user is not admin and not blocked and is not bot
func isRegularUser(uPtr *gitlab.User) bool {
	if uPtr.IsAdmin {
		return false
	}
	if uPtr.State == "blocked" {
		return false
	}
	return true
}

func isRegularUserByName(user string) bool {
	for _, u := range allUsers {
		if u.Username == user {
			return isRegularUser(&u)
		}
	}
	log.Fatalf("User '%s' is not found among all users!")
	return false
}
