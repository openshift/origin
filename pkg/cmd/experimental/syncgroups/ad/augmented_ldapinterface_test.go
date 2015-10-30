package ad

import (
	"reflect"
	"testing"

	"github.com/go-ldap/ldap"
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/testclient"
)

const (
	groupsBaseDN     string = "ou=groups,dc=example,dc=com"
	groupsBaseFilter string = "objectClass=groupOfNames"

	testGroupDN string = "cn=testGroup," + groupsBaseDN
)

func newTestAugmentedADLDAPInterface(client ldap.Client) *AugmentedADLDAPInterface {
	// below are common test implementations of LDAPInterface fields
	userQuery := ldaputil.LDAPQuery{
		BaseDN:       usersBaseDN,
		Scope:        ldaputil.ScopeWholeSubtree,
		DerefAliases: ldaputil.DerefAliasesAlways,
		TimeLimit:    0,
		Filter:       usersBaseFilter,
	}
	groupMembershipAttributes := []string{"memberOf"}
	userNameAttributes := []string{"email"}
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

	return NewAugmentedADLDAPInterface(testclient.NewConfig(client),
		userQuery,
		groupMembershipAttributes,
		userNameAttributes,
		groupQuery,
		groupNameAttributes)
}

// newDefaultTestGroup returns a new LDAP entry with the following characteristics:
// dn: cn=testGroup,ou=groups,dc=example,dc=com
// cn: testGroup
func newDefaultTestGroup() *ldap.Entry {
	return ldap.NewEntry(testGroupDN, map[string][]string{"cn": {"testGroup"}})
}

func TestGroupEntryFor(t *testing.T) {
	var testCases = []struct {
		name           string
		cacheSeed      map[string]*ldap.Entry
		client         ldap.Client
		baseDNOverride string
		expectedError  error
		expectedEntry  *ldap.Entry
	}{
		{
			name: "cached entries",
			cacheSeed: map[string]*ldap.Entry{
				testGroupDN: newDefaultTestGroup(),
			},
			expectedError: nil,
			expectedEntry: newDefaultTestGroup(),
		},
		{
			name:           "search request error",
			baseDNOverride: "otherBaseDN",
			expectedError:  ldaputil.NewQueryOutOfBoundsError(testGroupDN, "otherBaseDN"),
			expectedEntry:  nil,
		},
		{
			name:          "search error",
			client:        testclient.NewMatchingSearchErrorClient(testclient.New(), testGroupDN, searchErr),
			expectedError: searchErr,
			expectedEntry: nil,
		},
		{
			name: "no error",
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
		ldapInterface := newTestAugmentedADLDAPInterface(testCase.client)
		if len(testCase.cacheSeed) > 0 {
			ldapInterface.cachedGroups = testCase.cacheSeed
		}
		if len(testCase.baseDNOverride) > 0 {
			ldapInterface.groupQuery.BaseDN = testCase.baseDNOverride
		}
		entry, err := ldapInterface.GroupEntryFor(testGroupDN)
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(entry, testCase.expectedEntry) {
			t.Errorf("%s: incorrect members returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedEntry, entry)
		}
	}
}
