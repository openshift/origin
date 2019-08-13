package ad

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"gopkg.in/ldap.v2"

	ldapquery "github.com/openshift/library-go/pkg/security/ldapquery"
	"github.com/openshift/library-go/pkg/security/ldaptestclient"
	"github.com/openshift/library-go/pkg/security/ldaputil"
)

func newTestAugmentedADLDAPInterface(client ldap.Client) *AugmentedADLDAPInterface {
	// below are common test implementations of LDAPInterface fields
	userQuery := ldapquery.LDAPQuery{
		BaseDN:       "ou=users,dc=example,dc=com",
		Scope:        ldaputil.ScopeWholeSubtree,
		DerefAliases: ldaputil.DerefAliasesAlways,
		TimeLimit:    0,
		Filter:       "objectClass=inetOrgPerson",
	}
	groupMembershipAttributes := []string{"memberOf"}
	userNameAttributes := []string{"cn"}
	groupQuery := ldapquery.LDAPQueryOnAttribute{
		LDAPQuery: ldapquery.LDAPQuery{
			BaseDN:       "ou=groups,dc=example,dc=com",
			Scope:        ldaputil.ScopeWholeSubtree,
			DerefAliases: ldaputil.DerefAliasesAlways,
			TimeLimit:    0,
			Filter:       "objectClass=groupOfNames",
		},
		QueryAttribute: "dn",
	}
	groupNameAttributes := []string{"cn"}

	return NewAugmentedADLDAPInterface(ldaptestclient.NewConfig(client),
		userQuery,
		groupMembershipAttributes,
		userNameAttributes,
		groupQuery,
		groupNameAttributes)
}

// newDefaultTestGroup returns a new LDAP entry with the given CN
func newTestGroup(CN string) *ldap.Entry {
	return ldap.NewEntry(fmt.Sprintf("cn=%s,ou=groups,dc=example,dc=com", CN), map[string][]string{"cn": {CN}})
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
				"cn=testGroup,ou=groups,dc=example,dc=com": newTestGroup("testGroup"),
			},
			expectedError: nil,
			expectedEntry: newTestGroup("testGroup"),
		},
		{
			name:           "search request error",
			baseDNOverride: "dc=foo",
			expectedError:  ldapquery.NewQueryOutOfBoundsError("cn=testGroup,ou=groups,dc=example,dc=com", "dc=foo"),
			expectedEntry:  nil,
		},
		{
			name:          "search error",
			client:        ldaptestclient.NewMatchingSearchErrorClient(ldaptestclient.New(), "cn=testGroup,ou=groups,dc=example,dc=com", errors.New("generic search error")),
			expectedError: errors.New("generic search error"),
			expectedEntry: nil,
		},
		{
			name: "no error",
			client: ldaptestclient.NewDNMappingClient(
				ldaptestclient.New(),
				map[string][]*ldap.Entry{
					"cn=testGroup,ou=groups,dc=example,dc=com": {newTestGroup("testGroup")},
				},
			),
			expectedError: nil,
			expectedEntry: newTestGroup("testGroup"),
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
		entry, err := ldapInterface.GroupEntryFor("cn=testGroup,ou=groups,dc=example,dc=com")
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(entry, testCase.expectedEntry) {
			t.Errorf("%s: incorrect members returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedEntry, entry)
		}
	}
}
