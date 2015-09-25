package syncgroups

import (
	"fmt"
	"time"

	"github.com/go-ldap/ldap"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
	ouserapi "github.com/openshift/origin/pkg/user/api"
)

// GroupSyncer runs a Sync job on Groups
type GroupSyncer interface {
	Sync() (errors []error)
}

// LDAPGroupSyncer sync Groups with records on an external LDAP server
type LDAPGroupSyncer struct {
	// Lists all groups to be synced
	GroupLister interfaces.LDAPGroupLister
	// Fetches a group and extracts object metainformation and membership list from a group
	GroupMemberExtractor interfaces.LDAPMemberExtractor
	// Maps an LDAP user entry to an OpenShift User's Name
	UserNameMapper interfaces.LDAPUserNameMapper
	// Maps an LDAP group enrty to an OpenShift Group's Name
	GroupNameMapper interfaces.LDAPGroupNameMapper
	// Allows the Syncer to search for OpenShift Groups
	GroupClient client.GroupInterface
	// Host stores the address:port of the LDAP server
	Host string
	// SyncExisting determines if the sync job will only sync groups that already exist
	SyncExisting bool
}

// Sync allows the LDAPGroupSyncer to be a GroupSyncer
func (s *LDAPGroupSyncer) Sync() []error {
	var errors []error
	// determine what to sync
	ldapGroupUIDs, err := s.GroupLister.ListGroups()
	if err != nil {
		errors = append(errors, err)
		return errors
	}

	for _, ldapGroupUID := range ldapGroupUIDs {
		// get membership data
		memberEntries, err := s.GroupMemberExtractor.ExtractMembers(ldapGroupUID)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// determine OpenShift Users' usernames for LDAP group members
		usernames, err := s.determineUsernames(memberEntries)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// update the OpenShift Group corresponding to this record
		err = s.updateGroup(ldapGroupUID, usernames)
		if err != nil {
			errors = append(errors, err)
		}

	}
	return errors
}

// determineUsers determines the OpenShift Users that correspond to a list of LDAP member entries
func (s *LDAPGroupSyncer) determineUsernames(members []*ldap.Entry) ([]string, error) {
	var usernames []string
	for _, member := range members {
		username, err := s.UserNameMapper.UserNameFor(member)
		if err != nil {
			return nil, err
		}
		usernames = append(usernames, username)
	}
	return usernames, nil
}

// updateGroup finds or creates the OpenShift Group that needs to be updated, updates its' data, then
// uses the GroupClient to update the Group record
func (s *LDAPGroupSyncer) updateGroup(ldapGroupUID string, usernames []string) error {
	// find OpenShift Group to update
	group, err := s.findGroup(ldapGroupUID)
	if err != nil {
		return err
	}

	// overwrite Group Users data
	group.Users = usernames

	// add LDAP-sync-specific annotations
	group.Annotations[ldaputil.LDAPUIDAnnotation] = ldapGroupUID
	group.Annotations[ldaputil.LDAPSyncTimeAnnotation] = ISO8601(time.Now())
	group.Annotations[ldaputil.LDAPURLAnnotation] = s.Host

	_, err = s.GroupClient.Update(group)
	return err
}

// findGroup finds the OpenShift Group for the LDAP group UID and ensures that the OpenShift Group found
// was created as a result of a previous LDAP sync from the same LDAP group.
func (s *LDAPGroupSyncer) findGroup(ldapGroupUID string) (*ouserapi.Group, error) {
	groupName, err := s.GroupNameMapper.GroupNameFor(ldapGroupUID)
	if err != nil {
		return nil, err
	}

	group, err := s.GroupClient.Get(groupName)
	if err != nil {
		if s.SyncExisting {
			return nil, fmt.Errorf("could not get group for name: %s", groupName)
		} else {
			//TODO(deads): Do not create group here
			newGroup := &ouserapi.Group{
				ObjectMeta: kapi.ObjectMeta{
					Name: groupName,
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: s.Host,
						ldaputil.LDAPUIDAnnotation: ldapGroupUID,
					},
				},
			}

			group, err = s.GroupClient.Create(newGroup)
			if err != nil {
				return nil, fmt.Errorf("could not create new group for name %s: %v", groupName, err)
			}
		}
	}

	url, exists := group.Annotations[ldaputil.LDAPURLAnnotation]
	if !exists || url != s.Host {
		return nil, fmt.Errorf("group %s's %s annotation did not match sync host: wanted %s, got %s",
			group.Name, ldaputil.LDAPURLAnnotation, s.Host, url)
	}
	uid, exists := group.Annotations[ldaputil.LDAPUIDAnnotation]
	if !exists || uid != ldapGroupUID {
		return nil, fmt.Errorf("group %s's %s annotation did not match LDAP UID: wanted %s, got %s",
			group.Name, ldaputil.LDAPUIDAnnotation, ldapGroupUID, uid)
	}
	return group, nil
}

// ISO8601 returns an ISO 6801 formatted string from a time.
func ISO8601(t time.Time) string {
	var tz string
	if zone, offset := t.Zone(); zone == "UTC" {
		tz = "Z"
	} else {
		tz = fmt.Sprintf("%03d00", offset/3600)
	}
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%s",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), tz)
}
