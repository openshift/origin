package rfc2307

import (
	"fmt"

	"gopkg.in/ldap.v2"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/groupdetector"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/interfaces"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/syncerror"
)

// NewLDAPInterface builds a new LDAPInterface using a schema-appropriate config
func NewLDAPInterface(clientConfig ldapclient.Config,
	groupQuery ldaputil.LDAPQueryOnAttribute,
	groupNameAttributes []string,
	groupMembershipAttributes []string,
	userQuery ldaputil.LDAPQueryOnAttribute,
	userNameAttributes []string,
	errorHandler syncerror.Handler) *LDAPInterface {

	return &LDAPInterface{
		clientConfig:              clientConfig,
		groupQuery:                groupQuery,
		groupNameAttributes:       groupNameAttributes,
		groupMembershipAttributes: groupMembershipAttributes,
		userQuery:                 userQuery,
		userNameAttributes:        userNameAttributes,
		cachedUsers:               map[string]*ldap.Entry{},
		cachedGroups:              map[string]*ldap.Entry{},
		errorHandler:              errorHandler,
	}
}

// LDAPInterface extracts the member list of an LDAP group entry from an LDAP server
// with first-class LDAP entries for groups. The LDAPInterface is *NOT* thread-safe.
type LDAPInterface struct {
	// clientConfig holds LDAP connection information
	clientConfig ldapclient.Config

	// groupQuery holds the information necessary to make an LDAP query for a specific first-class group entry on the LDAP server
	groupQuery ldaputil.LDAPQueryOnAttribute
	// groupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for an OpenShift group
	groupNameAttributes []string
	// groupMembershipAttributes defines which attributes on an LDAP group entry will be interpreted as its members ldapUserUID
	groupMembershipAttributes []string

	// userQuery holds the information necessary to make an LDAP query for a specific first-class user entry on the LDAP server
	userQuery ldaputil.LDAPQueryOnAttribute
	// UserNameAttributes defines which attributes on an LDAP user entry will be interpreted as its' name
	userNameAttributes []string

	// cachedGroups holds the result of group queries for later reference, indexed on group UID
	// e.g. this will map an LDAP group UID to the LDAP entry returned from the query made using it
	cachedGroups map[string]*ldap.Entry
	// cachedUsers holds the result of user queries for later reference, indexed on user UID
	// e.g. this will map an LDAP user UID to the LDAP entry returned from the query made using it
	cachedUsers map[string]*ldap.Entry

	// errorHandler handles errors that occur
	errorHandler syncerror.Handler
}

// The LDAPInterface must conform to the following interfaces
var _ interfaces.LDAPMemberExtractor = &LDAPInterface{}
var _ interfaces.LDAPGroupGetter = &LDAPInterface{}
var _ interfaces.LDAPGroupLister = &LDAPInterface{}

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
		if err == nil {
			members = append(members, memberEntry)
			continue
		}

		err = syncerror.NewMemberLookupError(ldapGroupUID, ldapMemberUID, err)
		handled, fatalErr := e.errorHandler.HandleError(err)
		if fatalErr != nil {
			return nil, fatalErr
		}

		if !handled {
			return nil, err
		}

	}
	return members, nil
}

// GroupEntryFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the LDAPInterface first, then sending an LDAP query if the cache did not contain the entry.
// This also satisfies the LDAPGroupGetter interface
func (e *LDAPInterface) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
	group, exists := e.cachedGroups[ldapGroupUID]
	if exists {
		return group, nil
	}

	searchRequest, err := e.groupQuery.NewSearchRequest(ldapGroupUID, e.requiredGroupAttributes())
	if err != nil {
		return nil, err
	}

	group, err = ldaputil.QueryForUniqueEntry(e.clientConfig, searchRequest)
	if err != nil {
		return nil, err
	}
	e.cachedGroups[ldapGroupUID] = group
	return group, nil
}

// ListGroups queries for all groups as configured with the common group filter and returns their
// LDAP group UIDs. This also satisfies the LDAPGroupLister interface
func (e *LDAPInterface) ListGroups() ([]string, error) {
	searchRequest := e.groupQuery.LDAPQuery.NewSearchRequest(e.requiredGroupAttributes())
	groups, err := ldaputil.QueryForEntries(e.clientConfig, searchRequest)
	if err != nil {
		return nil, err
	}

	ldapGroupUIDs := []string{}
	for _, group := range groups {
		ldapGroupUID := ldaputil.GetAttributeValue(group, []string{e.groupQuery.QueryAttribute})
		if len(ldapGroupUID) == 0 {
			return nil, fmt.Errorf("unable to find LDAP group UID for %s", group)
		}
		e.cachedGroups[ldapGroupUID] = group
		ldapGroupUIDs = append(ldapGroupUIDs, ldapGroupUID)
	}
	return ldapGroupUIDs, nil
}

func (e *LDAPInterface) requiredGroupAttributes() []string {
	allAttributes := sets.NewString(e.groupNameAttributes...) // these attributes will be used for a future openshift group name mapping
	allAttributes.Insert(e.groupMembershipAttributes...)      // these attribute are used for finding group members
	allAttributes.Insert(e.groupQuery.QueryAttribute)         // this is used for extracting the group UID (otherwise an entry isn't self-describing)

	return allAttributes.List()
}

// userEntryFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the LDAPInterface first, then sending an LDAP query if the cache did not contain the entry
func (e *LDAPInterface) userEntryFor(ldapUserUID string) (user *ldap.Entry, err error) {
	user, exists := e.cachedUsers[ldapUserUID]
	if exists {
		return user, nil
	}

	searchRequest, err := e.userQuery.NewSearchRequest(ldapUserUID, e.requiredUserAttributes())
	if err != nil {
		return nil, err
	}

	user, err = ldaputil.QueryForUniqueEntry(e.clientConfig, searchRequest)
	if err != nil {
		return nil, err
	}
	e.cachedUsers[ldapUserUID] = user
	return user, nil
}

func (e *LDAPInterface) requiredUserAttributes() []string {
	allAttributes := sets.NewString(e.userNameAttributes...) // these attributes will be used for a future openshift user name mapping
	allAttributes.Insert(e.userQuery.QueryAttribute)         // this is used for extracting the user UID (otherwise an entry isn't self-describing)

	return allAttributes.List()
}

// Exists determines if a group idenified with its LDAP group UID exists on the LDAP server
func (e *LDAPInterface) Exists(ldapGroupUID string) (bool, error) {
	return groupdetector.NewGroupBasedDetector(e).Exists(ldapGroupUID)
}
