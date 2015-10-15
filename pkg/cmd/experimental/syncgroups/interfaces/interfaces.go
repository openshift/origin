package interfaces

import "github.com/go-ldap/ldap"

// LDAPGroupLister lists the LDAP groups that need to be synced by a job. The LDAPGroupLister needs to
// be paired with an LDAPMemberExtractor that understands the format of the unique identifiers returned
// to represent the LDAP groups to be synced.
type LDAPGroupLister interface {
	ListGroups() (ldapGroupUIDs []string, err error)
}

// LDAPMemberExtractor retrieves member data about an LDAP group from the LDAP server.
type LDAPMemberExtractor interface {
	// ExtractMembers returns the list of LDAP first-class user entries that are members of the LDAP group
	// specified by the ldapGroupUID
	ExtractMembers(ldapGroupUID string) (members []*ldap.Entry, err error)
}

// LDAPGroupNameMapper maps a ldapGroupUID representing an LDAP group to the OpenShift Group name for the resource
type LDAPGroupNameMapper interface {
	GroupNameFor(ldapGroupUID string) (openShiftGroupName string, err error)
}

// LDAPUserNameMapper maps an LDAP entry representing an LDAP user to the OpenShift User name for the resource
type LDAPUserNameMapper interface {
	UserNameFor(ldapUser *ldap.Entry) (openShiftUserName string, err error)
}

// LDAPGroupGetter maps a ldapGroupUID to a first-class LDAP group entry
type LDAPGroupGetter interface {
	GroupEntryFor(ldapGroupUID string) (group *ldap.Entry, err error)
}

type LDAPGroupListerNameMapper interface {
	LDAPGroupLister
	LDAPGroupNameMapper
}
