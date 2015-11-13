package rfc2307

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-ldap/ldap"
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/testclient"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
)

var searchErr error = errors.New("generic search error")

const (
	defaultFilter    string = "(objectClass=*)"
	groupsBaseDN     string = "ou=groups,dc=example,dc=com"
	groupsBaseFilter string = "objectClass=groupOfNames"
	usersBaseDN      string = "ou=users,dc=example,dc=com"
	usersBaseFilter  string = "objectClass=inetOrgPerson"

	testGroupDN string = "cn=testGroup," + groupsBaseDN
	testUserDN  string = "cn=testUser," + usersBaseDN
)

func newTestLDAPInterface(client ldap.Client) *LDAPInterface {
	// below are common test implementations of LDAPInterface fields
	groupQuery := ldaputil.LDAPQueryOnAttribute{
		LDAPQuery: ldaputil.LDAPQuery{
			BaseDN:       groupsBaseDN,
			Scope:        ldaputil.ScopeWholeSubtree,
			DerefAliases: ldaputil.DerefAliasesAlways,
			TimeLimit:    0,
			Filter:       groupsBaseFilter,
		},
		QueryAttribute: "dn",
	}
	groupNameAttributes := []string{"cn"}
	groupMembershipAttributes := []string{"member"}
	userQuery := ldaputil.LDAPQueryOnAttribute{
		LDAPQuery: ldaputil.LDAPQuery{
			BaseDN:       usersBaseDN,
			Scope:        ldaputil.ScopeWholeSubtree,
			DerefAliases: ldaputil.DerefAliasesAlways,
			TimeLimit:    0,
			Filter:       usersBaseFilter,
		},
		QueryAttribute: "dn",
	}
	userNameAttributes := []string{"email"}

	return NewLDAPInterface(testclient.NewConfig(client),
		groupQuery,
		groupNameAttributes,
		groupMembershipAttributes,
		userQuery,
		userNameAttributes)
}

// newDefaultTestUser returns a new LDAP entry with the following characteristics:
// dn: cn=testUser,ou=users,dc=example,dc=com
// cn: testUser
// email: testEmail
func newDefaultTestUser() *ldap.Entry {
	return ldap.NewEntry(testUserDN, map[string][]string{"cn": {"testUser"}, "email": {"testEmail"}})
}

// newDefaultTestGroup returns a new LDAP entry with the following characteristics:
// dn: cn=testGroup,ou=groups,dc=example,dc=com
// cn: testGroup
// member: cn=testUser,ou=users,dc=example,dc=com
func newDefaultTestGroup() *ldap.Entry {
	return ldap.NewEntry(testGroupDN, map[string][]string{"cn": {"testGroup"}, "member": {testUserDN}})
}

// newVariantTestGroup returns a new LDAP entry with the following characteristics:
// dn: cn=testGroup,ou=groups,dc=example,dc=com
// member: cn=testUser,ou=users,dc=example,dc=com
// NOTE: this group does NOT have a common name
func newVariantTestGroup() *ldap.Entry {
	return ldap.NewEntry(testGroupDN, map[string][]string{"member": {testUserDN}})
}

func TestExtractMembers(t *testing.T) {
	var testCases = []struct {
		name            string
		client          ldap.Client
		expectedError   error
		expectedMembers []*ldap.Entry
	}{
		{
			name:            "group lookup errors",
			client:          testclient.NewMatchingSearchErrorClient(testclient.New(), testGroupDN, searchErr),
			expectedError:   searchErr,
			expectedMembers: nil,
		},
		{
			name: "member lookup errors",
			// this is a nested test client, the first nest tries to error on the user DN
			// the second nest attempts to give back from the DN mapping
			// the third nest is the default "safe" impl from ldaputil
			client: testclient.NewMatchingSearchErrorClient(
				testclient.NewDNMappingClient(
					testclient.New(),
					map[string][]*ldap.Entry{
						testGroupDN: {newDefaultTestGroup()},
					},
				),
				testUserDN,
				searchErr,
			),
			expectedError:   interfaces.NewMemberLookupError(testGroupDN, testUserDN, searchErr),
			expectedMembers: nil,
		},
		{
			name: "no errors",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					testGroupDN: {newDefaultTestGroup()},
					testUserDN:  {newDefaultTestUser()},
				},
			),
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{newDefaultTestUser()},
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestLDAPInterface(testCase.client)
		members, err := ldapInterface.ExtractMembers(testGroupDN)
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(members, testCase.expectedMembers) {
			t.Errorf("%s: incorrect members returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedMembers, members)
		}
	}
}

func TestGroupEntryFor(t *testing.T) {
	var testCases = []struct {
		name                string
		cacheSeed           map[string]*ldap.Entry
		queryBaseDNOverride string
		client              ldap.Client
		expectedError       error
		expectedEntry       *ldap.Entry
	}{
		{
			name:          "cached get",
			cacheSeed:     map[string]*ldap.Entry{testGroupDN: newDefaultTestGroup()},
			expectedError: nil,
			expectedEntry: newDefaultTestGroup(),
		},
		{
			name:                "search request failure",
			queryBaseDNOverride: "otherBaseDN",
			expectedError:       ldaputil.NewQueryOutOfBoundsError(testGroupDN, "otherBaseDN"),
			expectedEntry:       nil,
		},
		{
			name:          "query failure",
			client:        testclient.NewMatchingSearchErrorClient(testclient.New(), testGroupDN, searchErr),
			expectedError: searchErr,
			expectedEntry: nil,
		},
		{
			name: "no errors",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					testGroupDN: {newDefaultTestGroup()},
				},
			),
			expectedError: nil,
			expectedEntry: newDefaultTestGroup(),
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestLDAPInterface(testCase.client)
		if len(testCase.cacheSeed) > 0 {
			ldapInterface.cachedGroups = testCase.cacheSeed
		}
		if len(testCase.queryBaseDNOverride) > 0 {
			ldapInterface.groupQuery.BaseDN = testCase.queryBaseDNOverride
		}
		entry, err := ldapInterface.GroupEntryFor(testGroupDN)
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(entry, testCase.expectedEntry) {
			t.Errorf("%s: incorrect entry returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedEntry, entry)
		}
	}
}

func TestListGroups(t *testing.T) {
	var testCases = []struct {
		name              string
		client            ldap.Client
		groupUIDAttribute string
		expectedError     error
		expectedGroups    []string
	}{
		{
			name:           "query errors",
			client:         testclient.NewMatchingSearchErrorClient(testclient.New(), groupsBaseDN, searchErr),
			expectedError:  searchErr,
			expectedGroups: nil,
		},
		{
			name: "no UID on entry",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					groupsBaseDN: {newVariantTestGroup()},
				},
			),
			groupUIDAttribute: "cn",
			expectedError:     fmt.Errorf("unable to find LDAP group UID for %s", newVariantTestGroup()),
			expectedGroups:    nil,
		},
		{
			name: "no error",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					groupsBaseDN: {newDefaultTestGroup()},
				},
			),
			expectedError:  nil,
			expectedGroups: []string{testGroupDN},
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestLDAPInterface(testCase.client)
		if len(testCase.groupUIDAttribute) > 0 {
			ldapInterface.groupQuery.QueryAttribute = testCase.groupUIDAttribute
		}
		groupNames, err := ldapInterface.ListGroups()
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(groupNames, testCase.expectedGroups) {
			t.Errorf("%s: incorrect entry returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedGroups, groupNames)
		}
	}
}

func TestUserEntryFor(t *testing.T) {
	var testCases = []struct {
		name                string
		cacheSeed           map[string]*ldap.Entry
		queryBaseDNOverride string
		client              ldap.Client
		expectedError       error
		expectedEntry       *ldap.Entry
	}{
		{
			name:          "cached get",
			cacheSeed:     map[string]*ldap.Entry{testUserDN: newDefaultTestUser()},
			expectedError: nil,
			expectedEntry: newDefaultTestUser(),
		},
		{
			name:                "search request failure",
			queryBaseDNOverride: "otherBaseDN",
			expectedError:       ldaputil.NewQueryOutOfBoundsError(testUserDN, "otherBaseDN"),
			expectedEntry:       nil,
		},
		{
			name:          "query failure",
			client:        testclient.NewMatchingSearchErrorClient(testclient.New(), testUserDN, searchErr),
			expectedError: searchErr,
			expectedEntry: nil,
		},
		{
			name: "no errors",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					testUserDN: {newDefaultTestUser()},
				},
			),
			expectedError: nil,
			expectedEntry: newDefaultTestUser(),
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestLDAPInterface(testCase.client)
		if len(testCase.cacheSeed) > 0 {
			ldapInterface.cachedUsers = testCase.cacheSeed
		}
		if len(testCase.queryBaseDNOverride) > 0 {
			ldapInterface.userQuery.BaseDN = testCase.queryBaseDNOverride
		}
		entry, err := ldapInterface.userEntryFor(testUserDN)
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(entry, testCase.expectedEntry) {
			t.Errorf("%s: incorrect entry returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedEntry, entry)
		}
	}
}
