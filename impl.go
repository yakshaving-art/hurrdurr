package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"

	yaml "github.com/ghodss/yaml"
	gitlab "github.com/xanzy/go-gitlab"
)

var gitlabToken string
var gitlabClient *gitlab.Client
var cfg config

// those are for fast local validation as opposed to N api calls
var allUsers []gitlab.User
var allGroups []gitlab.Group

var validACL = map[string]int{
	"no":         0,
	"guest":      10,
	"reporter":   20,
	"developer":  30,
	"maintainer": 40,
	"owner":      50,
}

func load() {
	gitlabToken = readFromEnv("GITLAB_TOKEN")
	gitlabClient = gitlab.NewClient(nil, gitlabToken)
	applyBaseURL(readFromEnv("GITLAB_BASEURL"))

	// prefetch users and groups
	allUsers = getAllUsers()
	allGroups = getAllGroups()

}

type config struct {
	Groups []Group `json:"groups"`
}

type Group map[string]GroupAccess
type GroupAccess []Membership

// UnmarshalJSON for GroupAccess is able to replace it with dynamic members
func (gaPtr *GroupAccess) UnmarshalJSON(b []byte) error {
	var tmp []Membership
	//var qtmp []Membership
	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}
	for _, r := range tmp {
		if r.Query != "" {
			// no merging yet
			tmp = queryUsers(r.Query, r.AccessLevel)
			//qtmp = queryUsers(r.Query, r.AccessLevel)
			// and merge with tmp (order is important)
			//mtmp := GroupAccess(tmp)
			//mtmp.merge(qtmp)
			//tmp = mtmp
		}
	}
	*gaPtr = GroupAccess(tmp)
	return nil
}

func (gaPtr *GroupAccess) merge(overwrite GroupAccess) {
	fmt.Printf("merging\n'%v'\nwith\n'%v'\n", gaPtr, overwrite)
	// O(N**2), refactor!
	for _, m := range overwrite {
		isMember := false
		for _, g := range *gaPtr {
			if m.Username == g.Username {
				g.AccessLevel = m.AccessLevel
				isMember = true
				break
			}
		}
		if !isMember {
			*gaPtr = append(*gaPtr, m)
		}
	}
}

// stupid sexy to be refactored predefined filter
func queryUsers(q string, acl string) []Membership {
	if q != "__all_regular__" && q != "__infrastructure__" {
		log.Fatalf("query '%s' is not supported!", q)
	}
	// __all_regular__ users
	var result []Membership
	if q == "__all_regular__" {
		for _, u := range allUsers {
			if isRegularUser(&u) {
				result = append(result, Membership{
					Username:    u.Username,
					AccessLevel: acl,
				})
			}
		}
	} else if q == "__infrastructure__" {
		for _, u := range cfg.Groups[0]["infrastructure"] {
			result = append(result, Membership{
				Username:    u.Username,
				AccessLevel: acl,
			})
		}
	}
	return result
}

func (cPtr *config) read(path string) {
	y, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Fatalf("Can't open config file '%s'", path)
	}
	if err = yaml.Unmarshal(y, cPtr); err != nil {
		log.Fatalf("Can't unmarshall: '%+v'", err)
	}
}

func (cPtr *config) validate() {
	groupNames := map[string]bool{}
	// TODO: refactor this
	for _, g := range cPtr.Groups {
		g.validate()
		for _, ga := range g {
			for _, m := range ga {
				m.validate()
			}
		}
		// check for duplicates
		for name := range g {
			if _, ok := groupNames[name]; ok {
				log.Fatalf("Error: duplicate group entry '%s'!", name)
			}
			groupNames[name] = true
		}
	}
}

func (cPtr *config) report() {
	// TODO: ugly warning, refactor this
	managedGroups := []string{}
	for _, g := range cPtr.Groups {
		for name := range g {
			managedGroups = append(managedGroups, name)
			break
		}
	}
	sort.Strings(managedGroups)

	fmt.Println("\nGroups not managed by this tool:")
	for _, g := range allGroups {
		if i := sort.SearchStrings(managedGroups, g.FullPath); i < len(managedGroups) && managedGroups[i] == g.FullPath {
			// already managed
		} else {
			fmt.Printf("\t%s\n", g.FullPath)
		}
	}
}

func (cPtr *config) apply() {
	for _, g := range cPtr.Groups {
		g.apply()
	}
}

func (gPtr *Group) validate() {
	// TODO: ugly warning, refactor
	if len(*gPtr) > 1 {
		log.Fatalf("data format error for map '%s'!", *gPtr)
	}
	var name string
	for key := range *gPtr {
		name = key
		break
	}
	// fmt.Printf("Validating group %s\n", name)
	// TODO: this is O(N**2), refactor (allgroups to map maybe?)
	validGroup := false
	for _, g := range allGroups {
		if g.FullPath == name {
			validGroup = true
			break
		}
	}
	if !validGroup {
		log.Fatalf("Group '%s' is invalid, not found in all groups!", name)
	}
}

func (gPtr *Group) apply() {
	// TODO: ugly warning, refactor, see above
	var fullPath string
	for key := range *gPtr {
		fullPath = key
		break
	}
	fmt.Printf("Applying group '%s'\n", fullPath)
	currentMembers := gitlabGetGroupMembers(fullPath)

	// helper slices
	currentUL := []string{}
	desiredUL := []string{}
	desiredACL := map[string]int{}
	for _, u := range currentMembers {
		currentUL = append(currentUL, u.Username)
	}
	for _, ga := range *gPtr {
		for _, u := range ga {
			desiredUL = append(desiredUL, u.Username)
			desiredACL[u.Username] = validACL[u.AccessLevel]
		}
	}

	sort.Strings(currentUL)
	sort.Strings(desiredUL)

	//fmt.Printf("current: %s\n", currentUL)
	//fmt.Printf("desired: %s\n", desiredUL)

	// cleanup/update first
	for _, m := range currentMembers {
		if i := sort.SearchStrings(desiredUL, m.Username); i < len(desiredUL) && desiredUL[i] == m.Username {
			//already member, check if access level update needed
			if int(m.AccessLevel) != desiredACL[m.Username] {
				updateMembersACL(m.Username, fullPath, desiredACL[m.Username])
			}
		} else {
			// needs to be removed
			removeMemberFromGroup(m.Username, fullPath)
		}

	}

	// next, add users which are in config but not in current members
	for _, m := range desiredUL {
		if i := sort.SearchStrings(currentUL, m); i < len(currentUL) && currentUL[i] == m {
			// already member, pass
			fmt.Printf("\tNo change for user '%s' with access '%d'\n", m, desiredACL[m])
		} else {
			// needs to be added
			addMemberToGroup(m, fullPath, desiredACL[m])
		}
	}
}

type Membership struct {
	Username    string `json:"username"`
	AccessLevel string `json:"access_level"`
	Query       string `json:"query", omitempty` // to allow unmarshalling as different type
	//Expiration  *gitlab.ISOTime `json:"expiration",omitempty`
}

func (mPtr *Membership) validate() {
	// fmt.Printf("Validating membership %s\n", mPtr)
	// TODO: this is O(N**2), refactor
	validUser := false
	for _, u := range allUsers {
		if u.Username == mPtr.Username {
			validUser = true
			break
		}
	}
	if !validUser {
		log.Fatalf("User '%s' is not found in all users, invalid!", mPtr.Username)
	}
	if _, ok := validACL[mPtr.AccessLevel]; !ok {
		log.Fatalf("Access level '%s' is invalid!", mPtr.AccessLevel)
	}
}

//func (aPtr *Access) validate() {
//	if aPtr.Type != "group" && aPtr.Type != "project" {
//		log.Fatalf("Access type for path '%s' should be either group or project, got '%s'",
//			aPtr.Path, aPtr.Type)
//	}
//	if aPtr.Type == "group" {
//		g, _, err := gitlabClient.Groups.GetGroup(aPtr.Path)
//		if err != nil {
//			log.Fatalf("Error getting group by id '%s': '%s'",
//				aPtr.Path, err)
//		}
//		aPtr.ID = g.ID
//		// validate all members
//		//for _, member := range aPtr.Members {
//		//	member.validate()
//		//}
//	}
//	if aPtr.Type == "project" {
//		log.Fatalf("not implemented yet")
//	}
//}
//
//// Member holds individual user access level
//type Member struct {
//	Username    string          `json:"user"`
//	ExpiresAt   *gitlab.ISOTime `json:"expiration,omitempty"`
//	ID          int             `json:"id,omitempty"`
//	AccessLevel Level           `json:"access_level,omitempty"`
//}
//
//type Level int
//
//func (lPtr *Level) UnmarshalJSON(b []byte) error {
//	var tmp string
//	if err := json.Unmarshal(b, &tmp); err != nil {
//		return err
//	}
//	var t Level
//	switch tmp {
//	case "no":
//		t = 0
//	case "guest":
//		t = 10
//	case "reporter":
//		t = 20
//	case "developer":
//		t = 30
//	case "maintainer":
//		t = 40
//	case "owner":
//		t = 50
//	default:
//		log.Fatalf("Unsupported level '%s'", tmp)
//	}
//	lPtr = &t
//	return nil
//}
//
//func (mPtr *Member) validate() {
//	fmt.Printf("Validating: %+v\n", mPtr)
//}
//
//type Members []Member
//
//// UnmarshalJSON for array of members, able to dynamically redefine it
//func (mPtr *Members) UnmarshalJSON(b []byte) error {
//	var tmp []map[string]string
//	if err := json.Unmarshal(b, &tmp); err != nil {
//		return err
//	}
//
//	// unmarshall individual entries
//	for _, r := range tmp {
//		if r["user"] != "" {
//			// first, try unmarshalling as Member
//			// ugly warning TODO: rewrite this shit ffs
//			data := fmt.Sprintf(`{"username":"%s"`, r["user"])
//			if r["access_level"] != "" {
//				data += fmt.Sprintf(`,"access_level":"%s"`, r["access_level"])
//			}
//			if r["expiration"] != "" {
//				data += fmt.Sprintf(`,"expiration":"%s"`, r["expiration"])
//			}
//			data += `}`
//			var tmp Member
//			if err := json.Unmarshal([]byte(data), &tmp); err != nil {
//				log.Fatalf("\n\n%s\n\n", data)
//				return err
//			}
//			*mPtr = append(*mPtr, tmp)
//		} else if r["group"] != "" {
//			// next, if group, fetch from gitlab
//			fmt.Printf("	group query: %#+v\n", r)
//			for _, m := range gitlabGetGroupMembers(r["group"]) {
//				*mPtr = append(*mPtr, m)
//			}
//		} else if r["dynamic"] != "" {
//			// next, if dynamic, fetch from gitlab
//			fmt.Printf("	dynamic query: %#+v\n", r)
//		} else {
//			log.Fatalf("Spec '%s' is not supported now\n", r)
//		}
//
//	}
//
//	//fmt.Printf("result: %+v\n", mPtr)
//	return nil
//}
//
