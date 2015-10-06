package rfc2307

import (
	"fmt"

	"github.com/go-ldap/ldap"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	ldapinterfaces "github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
)

// NewLDAPInterface builds a new LDAPInterface using a schema-appropriate config
func NewLDAPInterface(clientConfig ldaputil.LDAPClientConfig,
	groupQuery ldaputil.LDAPQueryOnAttribute,
	groupNameAttributes []string,
	groupMembershipAttributes []string,
	userQuery ldaputil.LDAPQueryOnAttribute,
	userNameAttributes []string) LDAPInterface {
	return LDAPInterface{
		clientConfig:              clientConfig,
		groupQuery:                groupQuery,
		groupNameAttributes:       groupNameAttributes,
		groupMembershipAttributes: groupMembershipAttributes,
		userQuery:                 userQuery,
		userNameAttributes:        userNameAttributes,
		cachedUsers:               make(map[string]*ldap.Entry),
		cachedGroups:              make(map[string]*ldap.Entry),
	}
}

// LDAPInterface extracts the member list of an LDAP group entry from an LDAP server
// with first-class LDAP entries for groups. The LDAPInterface is *NOT* thread-safe.
// The LDAPInterface satisfies:
// - LDAPMemberExtractor
// - LDAPGroupGetter
// - LDAPGroupLister
type LDAPInterface struct {
	// clientConfig holds LDAP connection information
	clientConfig ldaputil.LDAPClientConfig

	// groupQuery holds the information necessary to make an LDAP query for a specific
	// first-class group entry on the LDAP server
	groupQuery ldaputil.LDAPQueryOnAttribute
	// groupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for an OpenShift group
	groupNameAttributes []string
	// groupMembershipAttributes defines which attributes on an LDAP group entry will be interpreted as its members ldapUserUID
	groupMembershipAttributes []string

	// userQuery holds the information necessary to make an LDAP query for a specific
	// first-class user entry on the LDAP server
	userQuery ldaputil.LDAPQueryOnAttribute
	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its' name
	userNameAttributes []string

	// cachedGroups holds the result of group queries for later reference, indexed on group UID
	// e.g. this will map an LDAP group UID to the LDAP entry returned from the query made using it
	cachedGroups map[string]*ldap.Entry
	// cachedUsers holds the result of user queries for later reference, indexed on user UID
	// e.g. this will map an LDAP user UID to the LDAP entry returned from the query made using it
	cachedUsers map[string]*ldap.Entry
}

var _ ldapinterfaces.LDAPMemberExtractor = &LDAPInterface{}
var _ ldapinterfaces.LDAPGroupGetter = &LDAPInterface{}
var _ ldapinterfaces.LDAPGroupLister = &LDAPInterface{}

func (e *LDAPInterface) String() string {
	return fmt.Sprintf("%#v", e)
}

// ExtractMembers returns the LDAP member entries for a group specified with a ldapGroupUID
func (e *LDAPInterface) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	// get group entry from LDAP
	group, err := e.GroupEntryFor(ldapGroupUID)
	if err != nil {
		return nil, err
	}

	// extract member UIDs from group entry
	var ldapMemberUIDs []string
	for _, attribute := range e.groupMembershipAttributes {
		ldapMemberUIDs = append(ldapMemberUIDs, group.GetAttributeValues(attribute)...)
	}

	members := []*ldap.Entry{}
	// find members on LDAP server or in cache
	for _, ldapMemberUID := range ldapMemberUIDs {
		memberEntry, err := e.userEntryFor(ldapMemberUID)
		if err != nil {
			return nil, &ldapinterfaces.MemberLookupError{ldapGroupUID, ldapMemberUID, err}
		}
		members = append(members, memberEntry)
	}
	return members, nil
}

// GroupFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the LDAPInterface first, then sending an LDAP query if the cache did not contain the entry.
// This also satisfies the LDAPGroupGetter interface
func (e *LDAPInterface) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
	group, exists := e.cachedGroups[ldapGroupUID]
	if exists {
		return group, nil
	}

	group, err := e.queryForGroup(ldapGroupUID)
	if err != nil {
		return nil, err
	}
	// cache for annotation extraction
	e.cachedGroups[ldapGroupUID] = group
	return group, nil
}

// queryForGroup queries for a specific group identified by a ldapGroupUID with the query config stored
// in a LDAPInterface
func (e *LDAPInterface) queryForGroup(ldapGroupUID string) (*ldap.Entry, error) {
	// create the search request
	searchRequest, err := e.groupQuery.NewSearchRequest(ldapGroupUID, e.requiredGroupAttributes())
	if err != nil {
		return nil, err
	}

	return ldaputil.QueryForUniqueEntry(e.clientConfig, searchRequest)
}

// userEntryFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the LDAPInterface first, then sending an LDAP query if the cache did not contain the entry
func (e *LDAPInterface) userEntryFor(ldapUserUID string) (user *ldap.Entry, err error) {
	user, exists := e.cachedUsers[ldapUserUID]
	if !exists {
		user, err = e.queryForUser(ldapUserUID)
		if err != nil {
			return nil, err
		}
		// cache for annotation extraction
		e.cachedUsers[ldapUserUID] = user
	}
	return user, nil
}

// queryForUser queries for an LDAP user entry identified with an LDAP user UID on an LDAP server
// determined from a clientConfig by creating a search request from an LDAP query template and
// determining which attributes to search for with a LDAPuserAttributeDefiner
func (e *LDAPInterface) queryForUser(ldapUserUID string) (*ldap.Entry, error) {
	// create the search request
	searchRequest, err := e.userQuery.NewSearchRequest(ldapUserUID, e.requiredUserAttributes())
	if err != nil {
		return nil, err
	}

	return ldaputil.QueryForUniqueEntry(e.clientConfig, searchRequest)
}

// ListGroups queries for all groups as configured with the common group filter and returns their
// LDAP group UIDs. This also satisfies the LDAPGroupLister interface
func (e *LDAPInterface) ListGroups() ([]string, error) {
	groups, err := e.queryForGroups()
	if err != nil {
		return nil, err
	}

	ldapGroupUIDs := []string{}
	for _, group := range groups {
		// cache groups returned from the server for later
		ldapGroupUID := ldaputil.GetAttributeValue(group, []string{e.groupQuery.QueryAttribute})
		if len(ldapGroupUID) == 0 {
			return nil, fmt.Errorf("unable to find LDAP group UID for %v", group)
		}
		e.cachedGroups[ldapGroupUID] = group
		ldapGroupUIDs = append(ldapGroupUIDs, ldapGroupUID)
	}
	return ldapGroupUIDs, nil
}

// queryForGroups queries for all groups identified by a common filter in the query config stored
// in a GroupListerDataExtractor
func (e *LDAPInterface) queryForGroups() ([]*ldap.Entry, error) {
	// create the search request
	searchRequest := e.groupQuery.LDAPQuery.NewSearchRequest(e.requiredGroupAttributes())
	return ldaputil.QueryForEntries(e.clientConfig, searchRequest)
}

func (e *LDAPInterface) requiredGroupAttributes() []string {
	allAttributes := sets.NewString(e.groupNameAttributes...) // these attributes will be used for a future openshift group name mapping
	allAttributes.Insert(e.groupMembershipAttributes...)      // these attribute are used for finding group members
	allAttributes.Insert(e.groupQuery.QueryAttribute)         // this is used for extracting the group UID (otherwise an entry isn't self-describing)

	return allAttributes.List()
}

func (e *LDAPInterface) requiredUserAttributes() []string {
	allAttributes := sets.NewString(e.userNameAttributes...) // these attributes will be used for a future openshift user name mapping
	allAttributes.Insert(e.userQuery.QueryAttribute)         // this is used for extracting the user UID (otherwise an entry isn't self-describing)

	return allAttributes.List()
}
