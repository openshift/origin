package ad

import (
	"errors"
	"reflect"
	"testing"

	"github.com/go-ldap/ldap"
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/testclient"
)

var searchErr error = errors.New("generic search error")

const (
	defaultFilter   string = "(objectClass=*)"
	usersBaseDN     string = "ou=users,dc=example,dc=com"
	usersBaseFilter string = "objectClass=inetOrgPerson"

	testUserDN         string = "cn=testUser," + usersBaseDN
	testSecondUserDN   string = "cn=testUser2," + usersBaseDN
	testGroupUID       string = "testGroup"
	testSecondGroupUID string = "testSecondGroup"
)

func newTestADLDAPInterface(client ldap.Client) *ADLDAPInterface {
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

	return NewADLDAPInterface(testclient.NewConfig(client),
		userQuery,
		groupMembershipAttributes,
		userNameAttributes)
}

// newDefaultTestUser returns a new LDAP entry with the following characteristics:
// dn: cn=testUser,ou=users,dc=example,dc=com
// cn: testUser
// email: testEmail
// memberOf: testGroup
func newDefaultTestUser() *ldap.Entry {
	return ldap.NewEntry(testUserDN, map[string][]string{"cn": {"testUser"}, "email": {"testEmail"}, "memberOf": {testGroupUID}})
}

// newDefaultTestUser returns a new LDAP entry with the following characteristics:
// dn: cn=testUser2,ou=users,dc=example,dc=com
// cn: testUser2
// email: testEmail
// memberOf: testSecondGroup
func newVariantTestUser() *ldap.Entry {
	return ldap.NewEntry(testUserDN, map[string][]string{"cn": {"testUser2"}, "email": {"testEmail"}, "memberOf": {testSecondGroupUID}})
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
				testGroupUID: {newDefaultTestUser()},
			},
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{newDefaultTestUser()},
		},
		{
			name:            "user query error",
			client:          testclient.NewMatchingSearchErrorClient(testclient.New(), usersBaseDN, searchErr),
			expectedError:   searchErr,
			expectedMembers: nil,
		},
		{
			name: "no errors",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					usersBaseDN: {newDefaultTestUser()},
				},
			),
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{newDefaultTestUser()},
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestADLDAPInterface(testCase.client)
		if len(testCase.cacheSeed) > 0 {
			ldapInterface.ldapGroupToLDAPMembers = testCase.cacheSeed
		}
		members, err := ldapInterface.ExtractMembers(testGroupUID)
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
			usersBaseDN: {newDefaultTestUser()},
		},
	)
	ldapInterface := newTestADLDAPInterface(client)
	groups, err := ldapInterface.ListGroups()
	if !reflect.DeepEqual(err, nil) {
		t.Errorf("listing groups: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", nil, err)
	}
	if !reflect.DeepEqual(groups, []string{testGroupUID}) {
		t.Errorf("listing groups: incorrect group list:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", []string{testGroupUID}, groups)
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
				testGroupUID: {newDefaultTestUser()},
			},
			expectedError: nil,
			expectedCache: map[string][]*ldap.Entry{
				testGroupUID: {newDefaultTestUser()},
			},
		},
		{
			name:          "user query error",
			client:        testclient.NewMatchingSearchErrorClient(testclient.New(), usersBaseDN, searchErr),
			expectedError: searchErr,
			expectedCache: make(map[string][]*ldap.Entry), // won't be nil but will be empty
		},
		{
			name: "cache populated correctly",
			client: testclient.NewDNMappingClient(
				testclient.New(),
				map[string][]*ldap.Entry{
					usersBaseDN: {newDefaultTestUser()},
				},
			),
			expectedError: nil,
			expectedCache: map[string][]*ldap.Entry{
				testGroupUID: {newDefaultTestUser()},
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
			usersBaseDN: {newDefaultTestUser()},
		},
	)
	ldapInterface := newTestADLDAPInterface(client)
	_, err := ldapInterface.ExtractMembers(testGroupUID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// both queries use the same BaseDN so we change what the client returns to simulate not applying the group-specific filter
	client.(*testclient.DNMappingClient).DNMapping[usersBaseDN] = []*ldap.Entry{newDefaultTestUser(), newVariantTestUser()}

	expectedCache := map[string][]*ldap.Entry{
		testGroupUID:       {newDefaultTestUser()},
		testSecondGroupUID: {newVariantTestUser()},
	}

	err = ldapInterface.populateCache()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expectedCache, ldapInterface.ldapGroupToLDAPMembers) {
		t.Errorf("incorrect cache state:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", expectedCache, ldapInterface.ldapGroupToLDAPMembers)
	}
}
