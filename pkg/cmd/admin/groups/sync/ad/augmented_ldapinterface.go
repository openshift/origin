package ad

import (
	"gopkg.in/ldap.v2"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/groupdetector"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/interfaces"
)

// NewLDAPInterface builds a new LDAPInterface using a schema-appropriate config
func NewAugmentedADLDAPInterface(clientConfig ldapclient.Config,
	userQuery ldaputil.LDAPQuery,
	groupMembershipAttributes []string,
	userNameAttributes []string,
	groupQuery ldaputil.LDAPQueryOnAttribute,
	groupNameAttributes []string) *AugmentedADLDAPInterface {

	return &AugmentedADLDAPInterface{
		ADLDAPInterface:     NewADLDAPInterface(clientConfig, userQuery, groupMembershipAttributes, userNameAttributes),
		groupQuery:          groupQuery,
		groupNameAttributes: groupNameAttributes,
		cachedGroups:        map[string]*ldap.Entry{},
	}
}

// AugmentedADLDAPInterface extracts the member list of an LDAP user entry from an LDAP server
// with first-class LDAP entries for users and group.  Group membership is on the user. The AugmentedADLDAPInterface is *NOT* thread-safe.
type AugmentedADLDAPInterface struct {
	*ADLDAPInterface

	// groupQuery holds the information necessary to make an LDAP query for a specific
	// first-class group entry on the LDAP server
	groupQuery ldaputil.LDAPQueryOnAttribute
	// groupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for an OpenShift group
	groupNameAttributes []string

	// cachedGroups holds the result of group queries for later reference, indexed on group UID
	// e.g. this will map an LDAP group UID to the LDAP entry returned from the query made using it
	cachedGroups map[string]*ldap.Entry
}

var _ interfaces.LDAPMemberExtractor = &AugmentedADLDAPInterface{}
var _ interfaces.LDAPGroupGetter = &AugmentedADLDAPInterface{}
var _ interfaces.LDAPGroupLister = &AugmentedADLDAPInterface{}

// GroupEntryFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the AugmentedADLDAPInterface first, then sending an LDAP query if the cache did not contain the entry.
// This also satisfies the LDAPGroupGetter interface
func (e *AugmentedADLDAPInterface) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
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

	// cache for annotation extraction
	e.cachedGroups[ldapGroupUID] = group
	return group, nil
}

func (e *AugmentedADLDAPInterface) requiredGroupAttributes() []string {
	allAttributes := sets.NewString(e.groupNameAttributes...) // these attributes will be used for a future openshift group name mapping
	allAttributes.Insert(e.groupQuery.QueryAttribute)         // this is used for extracting the group UID (otherwise an entry isn't self-describing)

	return allAttributes.List()
}

// Exists determines if a group idenified with its LDAP group UID exists on the LDAP server
func (e *AugmentedADLDAPInterface) Exists(ldapGroupUID string) (bool, error) {
	groupDetector := groupdetector.NewCompoundDetector(groupdetector.NewGroupBasedDetector(e), groupdetector.NewMemberBasedDetector(e))
	return groupDetector.Exists(ldapGroupUID)
}
