package ad

import (
	"gopkg.in/ldap.v2"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/oc/admin/groups/sync/groupdetector"
	"github.com/openshift/origin/pkg/oc/admin/groups/sync/interfaces"
)

// NewADLDAPInterface builds a new ADLDAPInterface using a schema-appropriate config
func NewADLDAPInterface(clientConfig ldapclient.Config,
	userQuery ldaputil.LDAPQuery,
	groupMembershipAttributes []string,
	userNameAttributes []string) *ADLDAPInterface {

	return &ADLDAPInterface{
		clientConfig:              clientConfig,
		userQuery:                 userQuery,
		userNameAttributes:        userNameAttributes,
		groupMembershipAttributes: groupMembershipAttributes,
		ldapGroupToLDAPMembers:    map[string][]*ldap.Entry{},
	}
}

// ADLDAPInterface extracts the member list of an LDAP group entry from an LDAP server
// with first-class LDAP entries for user only. The ADLDAPInterface is *NOT* thread-safe.
type ADLDAPInterface struct {
	// clientConfig holds LDAP connection information
	clientConfig ldapclient.Config

	// userQuery holds the information necessary to make an LDAP query for all first-class user entries on the LDAP server
	userQuery ldaputil.LDAPQuery
	// groupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted as its ldapGroupUID
	groupMembershipAttributes []string
	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its name
	userNameAttributes []string

	// cacheFullyPopulated determines if the cache has been fully populated
	// populateCache() will populate it fully, specific calls to ExtractMembers() will not
	cacheFullyPopulated bool
	// ldapGroupToLDAPMembers holds the result of user queries for later reference, indexed on group UID
	// e.g. this will map all LDAP users to the LDAP group UID whose entry returned them
	ldapGroupToLDAPMembers map[string][]*ldap.Entry
}

// The LDAPInterface must conform to the following interfaces
var _ interfaces.LDAPMemberExtractor = &ADLDAPInterface{}
var _ interfaces.LDAPGroupLister = &ADLDAPInterface{}

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
		currQuery := ldaputil.LDAPQueryOnAttribute{LDAPQuery: e.userQuery, QueryAttribute: currAttribute}

		searchRequest, err := currQuery.NewSearchRequest(ldapGroupUID, e.requiredUserAttributes())
		if err != nil {
			return nil, err
		}

		currEntries, err := ldaputil.QueryForEntries(e.clientConfig, searchRequest)
		if err != nil {
			return nil, err
		}

		for _, currEntry := range currEntries {
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

	return sets.StringKeySet(e.ldapGroupToLDAPMembers).List(), nil
}

// populateCache queries all users to build a map of all the groups.  If the cache has already been
// populated, this is a no-op.
func (e *ADLDAPInterface) populateCache() error {
	if e.cacheFullyPopulated {
		return nil
	}

	searchRequest := e.userQuery.NewSearchRequest(e.requiredUserAttributes())

	userEntries, err := ldaputil.QueryForEntries(e.clientConfig, searchRequest)
	if err != nil {
		return err
	}

	for _, userEntry := range userEntries {
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
	e.cacheFullyPopulated = true

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
	allAttributes := sets.NewString(e.userNameAttributes...)
	allAttributes.Insert(e.groupMembershipAttributes...)

	return allAttributes.List()
}

// Exists determines if a group idenified with its LDAP group UID exists on the LDAP server
func (e *ADLDAPInterface) Exists(ldapGrouUID string) (bool, error) {
	return groupdetector.NewMemberBasedDetector(e).Exists(ldapGrouUID)
}
