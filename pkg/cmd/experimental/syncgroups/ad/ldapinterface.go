package ad

import (
	"reflect"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/go-ldap/ldap"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	ldapinterfaces "github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
)

// NewADLDAPInterface builds a new ADLDAPInterface using a schema-appropriate config
func NewADLDAPInterface(clientConfig ldaputil.LDAPClientConfig,
	userQuery ldaputil.LDAPQueryOnAttribute,
	groupMembershipAttributes []string,
	userNameAttributes []string) ADLDAPInterface {

	return ADLDAPInterface{
		clientConfig:              clientConfig,
		userQuery:                 userQuery,
		userNameAttributes:        userNameAttributes,
		groupMembershipAttributes: groupMembershipAttributes,
		ldapGroupToLDAPMembers:    map[string][]*ldap.Entry{},
	}
}

// ADLDAPInterface extracts the member list of an LDAP group entry from an LDAP server
// with first-class LDAP entries for user only. The ADLDAPInterface is *NOT* thread-safe.
// The ADLDAPInterface satisfies:
// - LDAPMemberExtractor
// - LDAPGroupLister
type ADLDAPInterface struct {
	// clientConfig holds LDAP connection information
	clientConfig ldaputil.LDAPClientConfig

	// userQuery holds the information necessary to make an LDAP query for a specific
	// first-class user entry on the LDAP server
	userQuery ldaputil.LDAPQueryOnAttribute
	// groupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted as its ldapGroupUID
	groupMembershipAttributes []string
	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its name
	userNameAttributes []string

	cachePopulated         bool
	ldapGroupToLDAPMembers map[string][]*ldap.Entry
}

var _ ldapinterfaces.LDAPMemberExtractor = &ADLDAPInterface{}
var _ ldapinterfaces.LDAPGroupLister = &ADLDAPInterface{}

// ExtractMembers returns the LDAP member entries for a group specified with a ldapGroupUID
func (e *ADLDAPInterface) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	// if we already have it cached, return the cached value
	if members, present := e.ldapGroupToLDAPMembers[ldapGroupUID]; present {
		return members, nil
	}

	// This happens in cases where we did not list out every group.  In that case, we're going to be asked about specific groups.
	usersInGroup := []*ldap.Entry{}

	// check for all users with ldapGroupUID in any of the allowed member attributes
	for _, currAttribute := range e.groupMembershipAttributes {
		currQuery := e.userQuery
		currQuery.QueryAttribute = currAttribute

		searchRequest, err := currQuery.NewSearchRequest(ldapGroupUID, e.requiredUserAttributes())
		if err != nil {
			return nil, err
		}

		currEntries, err := ldaputil.QueryForEntries(e.clientConfig, searchRequest)
		if err != nil {
			return nil, err
		}

		for i := range currEntries {
			currEntry := currEntries[i]

			if !isEntryPresent(usersInGroup, currEntry) {
				usersInGroup = append(usersInGroup, currEntry)
			}
		}
	}

	e.ldapGroupToLDAPMembers[ldapGroupUID] = usersInGroup

	return usersInGroup, nil
}

// ListGroups queries for all groups as configured with the common group filter and returns their
// LDAP group UIDs. This also satisfies the LDAPGroupLister interface
func (e *ADLDAPInterface) ListGroups() ([]string, error) {
	if err := e.populateCache(); err != nil {
		return nil, err
	}

	return sets.KeySet(reflect.ValueOf(e.ldapGroupToLDAPMembers)).List(), nil
}

// populateCache queries all users to build a map of all the groups.  If the cache has already been
// populated, this is a no-op.
func (e *ADLDAPInterface) populateCache() error {
	if e.cachePopulated {
		return nil
	}

	searchRequest := e.userQuery.LDAPQuery.NewSearchRequest(e.requiredUserAttributes())

	userEntries, err := ldaputil.QueryForEntries(e.clientConfig, searchRequest)
	if err != nil {
		return err
	}

	for i := range userEntries {
		userEntry := userEntries[i]
		if userEntry == nil {
			continue
		}

		for _, groupAttribute := range e.groupMembershipAttributes {
			for _, groupUID := range userEntry.GetAttributeValues(groupAttribute) {
				if _, exists := e.ldapGroupToLDAPMembers[groupUID]; !exists {
					e.ldapGroupToLDAPMembers[groupUID] = []*ldap.Entry{}
				}

				if !isEntryPresent(e.ldapGroupToLDAPMembers[groupUID], userEntry) {
					e.ldapGroupToLDAPMembers[groupUID] = append(e.ldapGroupToLDAPMembers[groupUID], userEntry)
				}
			}
		}
	}
	e.cachePopulated = true

	return nil
}

func isEntryPresent(haystack []*ldap.Entry, needle *ldap.Entry) bool {
	for _, curr := range haystack {
		if curr.DN == needle.DN {
			return true
		}
	}

	return false
}

func (e *ADLDAPInterface) requiredUserAttributes() []string {
	attributes := sets.NewString(e.groupMembershipAttributes...)
	attributes.Insert(e.userNameAttributes...)

	return attributes.List()
}
