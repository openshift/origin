package rfc2307

import (
	"errors"
	"fmt"
	"io/ioutil"
	"reflect"
	"testing"

	"gopkg.in/ldap.v2"

	ldapquery "github.com/openshift/library-go/pkg/security/ldapquery"
	"github.com/openshift/library-go/pkg/security/ldaptestclient"
	"github.com/openshift/library-go/pkg/security/ldaputil"
	"github.com/openshift/oc/pkg/helpers/groupsync/syncerror"
)

func newTestLDAPInterface(client ldap.Client) *LDAPInterface {
	// below are common test implementations of LDAPInterface fields
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
	groupMembershipAttributes := []string{"member"}
	userQuery := ldapquery.LDAPQueryOnAttribute{
		LDAPQuery: ldapquery.LDAPQuery{
			BaseDN:       "ou=users,dc=example,dc=com",
			Scope:        ldaputil.ScopeWholeSubtree,
			DerefAliases: ldaputil.DerefAliasesAlways,
			TimeLimit:    0,
			Filter:       "objectClass=inetOrgPerson",
		},
		QueryAttribute: "dn",
	}
	userNameAttributes := []string{"cn"}

	errorHandler := syncerror.NewCompoundHandler(
		syncerror.NewMemberLookupOutOfBoundsSuppressor(ioutil.Discard),
		syncerror.NewMemberLookupMemberNotFoundSuppressor(ioutil.Discard),
	)

	return NewLDAPInterface(ldaptestclient.NewConfig(client),
		groupQuery,
		groupNameAttributes,
		groupMembershipAttributes,
		userQuery,
		userNameAttributes,
		errorHandler)
}

// newTestUser returns a new LDAP entry with the CN
func newTestUser(CN string) *ldap.Entry {
	return ldap.NewEntry(fmt.Sprintf("cn=%s,ou=users,dc=example,dc=com", CN), map[string][]string{"cn": {CN}})
}

// newTestGroup returns a new LDAP entry with the given CN and member
func newTestGroup(CN, member string) *ldap.Entry {
	DN := fmt.Sprintf("cn=%s,ou=groups,dc=example,dc=com", CN)
	if len(CN) > 0 {
		return ldap.NewEntry(DN, map[string][]string{"cn": {CN}, "member": {member}})
	} else {
		// no CN
		return ldap.NewEntry(DN, map[string][]string{"member": {member}})
	}
}

func TestExtractMembers(t *testing.T) {
	var testCases = []struct {
		name            string
		client          ldap.Client
		expectedError   error
		expectedMembers []*ldap.Entry
	}{
		{
			name: "group lookup errors",
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.New(),
				"cn=testGroup,ou=groups,dc=example,dc=com",
				errors.New("generic search error"),
			),
			expectedError:   errors.New("generic search error"),
			expectedMembers: nil,
		},
		{
			name: "member lookup errors",
			// this is a nested test client, the first nest tries to error on the user DN
			// the second nest attempts to give back from the DN mapping
			// the third nest is the default "safe" impl from ldaputil
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.NewDNMappingClient(
					ldaptestclient.New(),
					map[string][]*ldap.Entry{
						"cn=testGroup,ou=groups,dc=example,dc=com": {newTestGroup("testGroup", "cn=testUser,ou=users,dc=example,dc=com")},
					},
				),
				"cn=testUser,ou=users,dc=example,dc=com",
				errors.New("generic search error"),
			),
			expectedError:   syncerror.NewMemberLookupError("cn=testGroup,ou=groups,dc=example,dc=com", "cn=testUser,ou=users,dc=example,dc=com", errors.New("generic search error")),
			expectedMembers: nil,
		},
		{
			name: "out of scope member lookup suppressed",
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.NewDNMappingClient(
					ldaptestclient.New(),
					map[string][]*ldap.Entry{
						"cn=testGroup,ou=groups,dc=example,dc=com": {newTestGroup("testGroup", "cn=testUser,ou=users,dc=other-example,dc=com")},
					},
				),
				"cn=testUser,ou=users,dc=other-example,dc=com",
				ldapquery.NewQueryOutOfBoundsError("cn=testUser,ou=users,dc=other-example,dc=com", "cn=testGroup,ou=groups,dc=example,dc=com"),
			),
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{},
		},
		{
			name: "no such object member lookup error suppressed",
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.NewDNMappingClient(
					ldaptestclient.New(),
					map[string][]*ldap.Entry{
						"cn=testGroup,ou=groups,dc=example,dc=com": {newTestGroup("testGroup", "cn=testUser,ou=users,dc=other-example,dc=com")},
					},
				),
				"cn=testUser,ou=users,dc=other-example,dc=com",
				ldapquery.NewNoSuchObjectError("cn=testUser,ou=users,dc=other-example,dc=com"),
			),
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{},
		},
		{
			name: "member not found member lookup error suppressed",
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.NewDNMappingClient(
					ldaptestclient.New(),
					map[string][]*ldap.Entry{
						"cn=testGroup,ou=groups,dc=example,dc=com": {newTestGroup("testGroup", "cn=testUser,ou=users,dc=other-example,dc=com")},
					},
				),
				"cn=testUser,ou=users,dc=other-example,dc=com",
				ldapquery.NewEntryNotFoundError("cn=testUser,ou=users,dc=other-example,dc=com", "objectClass=groupOfNames"),
			),
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{},
		},
		{
			name: "no errors",
			client: ldaptestclient.NewDNMappingClient(
				ldaptestclient.New(),
				map[string][]*ldap.Entry{
					"cn=testGroup,ou=groups,dc=example,dc=com": {newTestGroup("testGroup", "cn=testUser,ou=users,dc=example,dc=com")},
					"cn=testUser,ou=users,dc=example,dc=com":   {newTestUser("testUser")},
				},
			),
			expectedError:   nil,
			expectedMembers: []*ldap.Entry{newTestUser("testUser")},
		},
	}
	for _, testCase := range testCases {
		ldapInterface := newTestLDAPInterface(testCase.client)
		members, err := ldapInterface.ExtractMembers("cn=testGroup,ou=groups,dc=example,dc=com")
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
			cacheSeed:     map[string]*ldap.Entry{"cn=testGroup,ou=groups,dc=example,dc=com": newTestGroup("testGroup", "cn=testUser,ou=users,dc=example,dc=com")},
			expectedError: nil,
			expectedEntry: newTestGroup("testGroup", "cn=testUser,ou=users,dc=example,dc=com"),
		},
		{
			name:                "search request failure",
			queryBaseDNOverride: "dc=foo",
			expectedError:       ldapquery.NewQueryOutOfBoundsError("cn=testGroup,ou=groups,dc=example,dc=com", "dc=foo"),
			expectedEntry:       nil,
		},
		{
			name: "query failure",
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.New(),
				"cn=testGroup,ou=groups,dc=example,dc=com",
				errors.New("generic search error"),
			),
			expectedError: errors.New("generic search error"),
			expectedEntry: nil,
		},
		{
			name: "no errors",
			client: ldaptestclient.NewDNMappingClient(
				ldaptestclient.New(),
				map[string][]*ldap.Entry{
					"cn=testGroup,ou=groups,dc=example,dc=com": {newTestGroup("testGroup", "cn=testUser,ou=users,dc=example,dc=com")},
				},
			),
			expectedError: nil,
			expectedEntry: newTestGroup("testGroup", "cn=testUser,ou=users,dc=example,dc=com"),
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
		entry, err := ldapInterface.GroupEntryFor("cn=testGroup,ou=groups,dc=example,dc=com")
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
			name: "query errors",
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.New(),
				"ou=groups,dc=example,dc=com",
				errors.New("generic search error"),
			),
			expectedError:  errors.New("generic search error"),
			expectedGroups: nil,
		},
		{
			name: "no UID on entry",
			client: ldaptestclient.NewDNMappingClient(
				ldaptestclient.New(),
				map[string][]*ldap.Entry{
					"ou=groups,dc=example,dc=com": {newTestGroup("", "cn=testUser,ou=users,dc=example,dc=com")},
				},
			),
			groupUIDAttribute: "cn",
			expectedError:     fmt.Errorf("unable to find LDAP group UID for %s", newTestGroup("", "cn=testUser,ou=users,dc=example,dc=com").DN),
			expectedGroups:    nil,
		},
		{
			name: "no error",
			client: ldaptestclient.NewDNMappingClient(
				ldaptestclient.New(),
				map[string][]*ldap.Entry{
					"ou=groups,dc=example,dc=com": {newTestGroup("testGroup", "cn=testUser,ou=users,dc=example,dc=com")},
				},
			),
			expectedError:  nil,
			expectedGroups: []string{"cn=testGroup,ou=groups,dc=example,dc=com"},
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
			name: "cached get",
			cacheSeed: map[string]*ldap.Entry{
				"cn=testUser,ou=users,dc=example,dc=com": newTestUser("testUser"),
			},
			expectedError: nil,
			expectedEntry: newTestUser("testUser"),
		},
		{
			name:                "search request failure",
			queryBaseDNOverride: "dc=foo",
			expectedError:       ldapquery.NewQueryOutOfBoundsError("cn=testUser,ou=users,dc=example,dc=com", "dc=foo"),
			expectedEntry:       nil,
		},
		{
			name: "query failure",
			client: ldaptestclient.NewMatchingSearchErrorClient(
				ldaptestclient.New(),
				"cn=testUser,ou=users,dc=example,dc=com",
				errors.New("generic search error"),
			),
			expectedError: errors.New("generic search error"),
			expectedEntry: nil,
		},
		{
			name: "no errors",
			client: ldaptestclient.NewDNMappingClient(
				ldaptestclient.New(),
				map[string][]*ldap.Entry{
					"cn=testUser,ou=users,dc=example,dc=com": {newTestUser("testUser")},
				},
			),
			expectedError: nil,
			expectedEntry: newTestUser("testUser"),
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
		entry, err := ldapInterface.userEntryFor("cn=testUser,ou=users,dc=example,dc=com")
		if !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedError, err)
		}
		if !reflect.DeepEqual(entry, testCase.expectedEntry) {
			t.Errorf("%s: incorrect entry returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedEntry, entry)
		}
	}
}
