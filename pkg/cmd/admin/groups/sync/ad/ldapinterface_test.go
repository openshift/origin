package ad

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"gopkg.in/ldap.v2"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/testclient"
)

func newTestADLDAPInterface(client ldap.Client) *ADLDAPInterface {
	// below are common test implementations of LDAPInterface fields
	userQuery := ldaputil.LDAPQuery{
		BaseDN:       "ou=users,dc=example,dc=com",
		Scope:        ldaputil.ScopeWholeSubtree,
		DerefAliases: ldaputil.DerefAliasesAlways,
		TimeLimit:    0,
		Filter:       "objectClass=inetOrgPerson",
	}
	groupMembershipAttributes := []string{"memberOf"}
	userNameAttributes := []string{"cn"}

	return NewADLDAPInterface(testclient.NewConfig(client),
		userQuery,
		groupMembershipAttributes,
		userNameAttributes)
}

// newTestUser(testUserDN, testUserCN, testGroupUID) returns a new LDAP entry with the given CN,
// as a member of the given group
func newTestUser(CN, groupUID string) *ldap.Entry {
	return ldap.NewEntry(fmt.Sprintf("cn=%s,ou=users,dc=example,dc=com", CN), map[string][]string{"cn": {CN}, "memberOf": {groupUID}})
}

func TestExtractMembers(t *testing.T) {
	// we don't have a test case for an error on a bad search request as search request errors can only occur if
	// the search attribute is the DN, and we do not allow DN to be a group UID for this schema
	var testCases = []struct {
		name            string
		cacheSeed       map[string][]*ldap.Entry
		client          ldap.Client
		expectedError   error
		expectedMembers []*ldap.Entry
	}{
		{
			name: "members cached",
			cacheSeed: map[string][]*ldap.Entry{
				"testGroup": {newTestUser("testUser", "testGroup")},
			},
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{newTestUser("testUser", "testGroup")},
		},
		{
			name: "user query error",
			client: testclient.NewMatchingSearchErrorClient(
				testclient.New(),
				"ou=users,dc=example,dc=com",
				errors.New("generic search error"),
			),
			expectedError:   errors.New("generic search error"),
			expectedMembers: nil,
		},
		{
			name: "no errors",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					"ou=users,dc=example,dc=com": {newTestUser("testUser", "testGroup")},
				},
			),
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{newTestUser("testUser", "testGroup")},
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestADLDAPInterface(testCase.client)
		if len(testCase.cacheSeed) > 0 {
			ldapInterface.ldapGroupToLDAPMembers = testCase.cacheSeed
		}
		members, err := ldapInterface.ExtractMembers("testGroup")
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(members, testCase.expectedMembers) {
			t.Errorf("%s: incorrect members returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedMembers, members)
		}
	}
}

func TestListGroups(t *testing.T) {
	client := testclient.NewDNMappingClient(
		testclient.New(),
		map[string][]*ldap.Entry{
			"ou=users,dc=example,dc=com": {newTestUser("testUser", "testGroup")},
		},
	)
	ldapInterface := newTestADLDAPInterface(client)
	groups, err := ldapInterface.ListGroups()
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("listing groups: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", nil, err)
	}
	if !reflect.DeepEqual(groups, []string{"testGroup"}) {
		t.Errorf("listing groups: incorrect group list:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", []string{"testGroup"}, groups)
	}
}

func TestPopulateCache(t *testing.T) {
	var testCases = []struct {
		name             string
		cacheSeed        map[string][]*ldap.Entry
		searchDNOverride string
		client           ldap.Client
		expectedError    error
		expectedCache    map[string][]*ldap.Entry
	}{
		{
			name: "cache already populated",
			cacheSeed: map[string][]*ldap.Entry{
				"testGroup": {newTestUser("testUser", "testGroup")},
			},
			expectedError: nil,
			expectedCache: map[string][]*ldap.Entry{
				"testGroup": {newTestUser("testUser", "testGroup")},
			},
		},
		{
			name: "user query error",
			client: testclient.NewMatchingSearchErrorClient(
				testclient.New(),
				"ou=users,dc=example,dc=com",
				errors.New("generic search error"),
			),
			expectedError: errors.New("generic search error"),
			expectedCache: make(map[string][]*ldap.Entry), // won't be nil but will be empty
		},
		{
			name: "cache populated correctly",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					"ou=users,dc=example,dc=com": {newTestUser("testUser", "testGroup")},
				},
			),
			expectedError: nil,
			expectedCache: map[string][]*ldap.Entry{
				"testGroup": {newTestUser("testUser", "testGroup")},
			},
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestADLDAPInterface(testCase.client)
		if len(testCase.cacheSeed) > 0 {
			ldapInterface.ldapGroupToLDAPMembers = testCase.cacheSeed
			ldapInterface.cacheFullyPopulated = true
		}
		err := ldapInterface.populateCache()
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(testCase.expectedCache, ldapInterface.ldapGroupToLDAPMembers) {
			t.Errorf("%s: incorrect cache state:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedCache, ldapInterface.ldapGroupToLDAPMembers)
		}
	}
}

// TestPopulateCacheAfterExtractMembers ensures that the cache is only listed as fully populated after a
// populateCache call and not after a partial fill from an ExtractMembers call
func TestPopulateCacheAfterExtractMembers(t *testing.T) {
	client := testclient.NewDNMappingClient(
		testclient.New(),
		map[string][]*ldap.Entry{
			"ou=users,dc=example,dc=com": {newTestUser("testUser", "testGroup")},
		},
	)
	ldapInterface := newTestADLDAPInterface(client)
	_, err := ldapInterface.ExtractMembers("testGroup")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// both queries use the same BaseDN so we change what the client returns to simulate not applying the group-specific filter
	client.(*testclient.DNMappingClient).DNMapping["ou=users,dc=example,dc=com"] = []*ldap.Entry{
		newTestUser("testUser", "testGroup"),
		newTestUser("testUser2", "testGroup2")}

	expectedCache := map[string][]*ldap.Entry{
		"testGroup":  {newTestUser("testUser", "testGroup")},
		"testGroup2": {newTestUser("testUser2", "testGroup2")},
	}

	err = ldapInterface.populateCache()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expectedCache, ldapInterface.ldapGroupToLDAPMembers) {
		t.Errorf("incorrect cache state:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", expectedCache, ldapInterface.ldapGroupToLDAPMembers)
	}
}
