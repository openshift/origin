package ad

import (
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/go-ldap/ldap"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	ldapinterfaces "github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
)

// NewLDAPInterface builds a new LDAPInterface using a schema-appropriate config
<<<<<<< HEAD:pkg/cmd/experimental/syncgroups/ad/enhanced_ldapinterface.go
func NewEnhancedADLDAPInterface(clientConfig ldaputil.LDAPClientConfig,
=======
func NewAugmentedADLDAPInterface(clientConfig *ldaputil.LDAPClientConfig,
>>>>>>> d67cc33... main:pkg/cmd/experimental/syncgroups/ad/augmented_ldapinterface.go
	userQuery ldaputil.LDAPQueryOnAttribute,
	groupMembershipAttributes []string,
	userNameAttributes []string,
	groupQuery ldaputil.LDAPQueryOnAttribute,
<<<<<<< HEAD:pkg/cmd/experimental/syncgroups/ad/enhanced_ldapinterface.go
	groupNameAttributes []string) EnhancedADLDAPInterface {

	return EnhancedADLDAPInterface{
=======
	groupNameAttributes []string) *AugmentedADLDAPInterface {

	return &AugmentedADLDAPInterface{
>>>>>>> d67cc33... main:pkg/cmd/experimental/syncgroups/ad/augmented_ldapinterface.go
		ADLDAPInterface:     NewADLDAPInterface(clientConfig, userQuery, groupMembershipAttributes, userNameAttributes),
		groupQuery:          groupQuery,
		groupNameAttributes: groupNameAttributes,
		cachedGroups:        map[string]*ldap.Entry{},
	}
}

// LDAPInterface extracts the member list of an LDAP user entry from an LDAP server
// with first-class LDAP entries for users and group.  Group membership is on the user. The LDAPInterface is *NOT* thread-safe.
// The LDAPInterface satisfies:
// - LDAPMemberExtractor
// - LDAPGroupGetter
// - LDAPGroupLister
<<<<<<< HEAD:pkg/cmd/experimental/syncgroups/ad/enhanced_ldapinterface.go
type EnhancedADLDAPInterface struct {
	ADLDAPInterface
=======
type AugmentedADLDAPInterface struct {
	*ADLDAPInterface
>>>>>>> d67cc33... main:pkg/cmd/experimental/syncgroups/ad/augmented_ldapinterface.go

	// groupQuery holds the information necessary to make an LDAP query for a specific
	// first-class group entry on the LDAP server
	groupQuery ldaputil.LDAPQueryOnAttribute
	// groupNameAttributes defines which attributes on an LDAP group entry will be interpreted as its name to use for an OpenShift group
	groupNameAttributes []string

	cachedGroups map[string]*ldap.Entry
}

var _ ldapinterfaces.LDAPMemberExtractor = &AugmentedADLDAPInterface{}
var _ ldapinterfaces.LDAPGroupGetter = &AugmentedADLDAPInterface{}
var _ ldapinterfaces.LDAPGroupLister = &AugmentedADLDAPInterface{}

// GroupFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the LDAPInterface first, then sending an LDAP query if the cache did not contain the entry.
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
