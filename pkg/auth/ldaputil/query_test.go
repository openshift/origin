package ldaputil

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/openshift/origin/pkg/auth/ldaputil/testclient"
	"gopkg.in/ldap.v2"
)

const (
	DefaultBaseDN         string       = "dc=example,dc=com"
	DefaultScope          Scope        = ScopeWholeSubtree
	DefaultDerefAliases   DerefAliases = DerefAliasesAlways
	DefaultSizeLimit      int          = 0
	DefaultTimeLimit      int          = 0
	DefaultTypesOnly      bool         = false
	DefaultFilter         string       = "objectClass=groupOfNames"
	DefaultQueryAttribute string       = "uid"
)

var DefaultAttributes []string = []string{"dn", "cn", "uid"}
var DefaultControls []ldap.Control = nil

func TestNewSearchRequest(t *testing.T) {
	var testCases = []struct {
		name            string
		options         LDAPQueryOnAttribute
		attributeValue  string
		attributes      []string
		expectedRequest *ldap.SearchRequest
		expectedError   bool
	}{
		{
			name: "attribute query no attributes",
			options: LDAPQueryOnAttribute{
				LDAPQuery: LDAPQuery{
					BaseDN:       DefaultBaseDN,
					Scope:        DefaultScope,
					DerefAliases: DefaultDerefAliases,
					TimeLimit:    DefaultTimeLimit,
					Filter:       DefaultFilter,
				},
				QueryAttribute: DefaultQueryAttribute,
			},

			attributeValue: "bar",
			attributes:     DefaultAttributes,
			expectedRequest: &ldap.SearchRequest{
				BaseDN:       DefaultBaseDN,
				Scope:        int(DefaultScope),
				DerefAliases: int(DefaultDerefAliases),
				SizeLimit:    DefaultSizeLimit,
				TimeLimit:    DefaultTimeLimit,
				TypesOnly:    DefaultTypesOnly,
				Filter:       fmt.Sprintf("(&(%s)(%s=%s))", DefaultFilter, DefaultQueryAttribute, "bar"),
				Attributes:   DefaultAttributes,
				Controls:     DefaultControls,
			},
			expectedError: false,
		},
		{
			name: "attribute query with additional attributes",
			options: LDAPQueryOnAttribute{
				LDAPQuery: LDAPQuery{
					BaseDN:       DefaultBaseDN,
					Scope:        DefaultScope,
					DerefAliases: DefaultDerefAliases,
					TimeLimit:    DefaultTimeLimit,
					Filter:       DefaultFilter,
				},
				QueryAttribute: DefaultQueryAttribute,
			},
			attributeValue: "bar",
			attributes:     append(DefaultAttributes, []string{"email", "phone"}...),
			expectedRequest: &ldap.SearchRequest{
				BaseDN:       DefaultBaseDN,
				Scope:        int(DefaultScope),
				DerefAliases: int(DefaultDerefAliases),
				SizeLimit:    DefaultSizeLimit,
				TimeLimit:    DefaultTimeLimit,
				TypesOnly:    DefaultTypesOnly,
				Filter:       fmt.Sprintf("(&(%s)(%s=%s))", DefaultFilter, DefaultQueryAttribute, "bar"),
				Attributes:   append(DefaultAttributes, []string{"email", "phone"}...),
				Controls:     DefaultControls,
			},
			expectedError: false,
		},
		{
			name: "valid dn query no attributes",
			options: LDAPQueryOnAttribute{
				LDAPQuery: LDAPQuery{
					BaseDN:       DefaultBaseDN,
					Scope:        DefaultScope,
					DerefAliases: DefaultDerefAliases,
					TimeLimit:    DefaultTimeLimit,
					Filter:       DefaultFilter,
				},
				QueryAttribute: "DN",
			},
			attributeValue: "uid=john,o=users,dc=example,dc=com",
			attributes:     DefaultAttributes,
			expectedRequest: &ldap.SearchRequest{
				BaseDN:       "uid=john,o=users,dc=example,dc=com",
				Scope:        ldap.ScopeBaseObject,
				DerefAliases: int(DefaultDerefAliases),
				SizeLimit:    DefaultSizeLimit,
				TimeLimit:    DefaultTimeLimit,
				TypesOnly:    DefaultTypesOnly,
				Filter:       "(objectClass=*)",
				Attributes:   DefaultAttributes,
				Controls:     DefaultControls,
			},
			expectedError: false,
		},
		{
			name: "valid dn query with additional attributes",
			options: LDAPQueryOnAttribute{
				LDAPQuery: LDAPQuery{
					BaseDN:       DefaultBaseDN,
					Scope:        DefaultScope,
					DerefAliases: DefaultDerefAliases,
					TimeLimit:    DefaultTimeLimit,
					Filter:       DefaultFilter,
				},
				QueryAttribute: "DN",
			},
			attributeValue: "uid=john,o=users,dc=example,dc=com",
			attributes:     append(DefaultAttributes, []string{"email", "phone"}...),
			expectedRequest: &ldap.SearchRequest{
				BaseDN:       "uid=john,o=users,dc=example,dc=com",
				Scope:        ldap.ScopeBaseObject,
				DerefAliases: int(DefaultDerefAliases),
				SizeLimit:    DefaultSizeLimit,
				TimeLimit:    DefaultTimeLimit,
				TypesOnly:    DefaultTypesOnly,
				Filter:       "(objectClass=*)",
				Attributes:   append(DefaultAttributes, []string{"email", "phone"}...),
				Controls:     DefaultControls,
			},
			expectedError: false,
		},
		{
			name: "invalid dn query out of bounds",
			options: LDAPQueryOnAttribute{
				LDAPQuery: LDAPQuery{
					BaseDN:       DefaultBaseDN,
					Scope:        DefaultScope,
					DerefAliases: DefaultDerefAliases,
					TimeLimit:    DefaultTimeLimit,
					Filter:       DefaultFilter,
				},
				QueryAttribute: "DN",
			},
			attributeValue:  "uid=john,o=users,dc=other,dc=com",
			attributes:      DefaultAttributes,
			expectedRequest: nil,
			expectedError:   true,
		},
		{
			name: "invalid dn query invalid dn",
			options: LDAPQueryOnAttribute{
				LDAPQuery: LDAPQuery{
					BaseDN:       DefaultBaseDN,
					Scope:        DefaultScope,
					DerefAliases: DefaultDerefAliases,
					TimeLimit:    DefaultTimeLimit,
					Filter:       DefaultFilter,
				},
				QueryAttribute: "DN",
			},
			attributeValue:  "uid=,o=users,dc=other,dc=com",
			attributes:      DefaultAttributes,
			expectedRequest: nil,
			expectedError:   true,
		},
		{
			name: "attribute query no attributes with paging",
			options: LDAPQueryOnAttribute{
				LDAPQuery: LDAPQuery{
					BaseDN:       DefaultBaseDN,
					Scope:        DefaultScope,
					DerefAliases: DefaultDerefAliases,
					TimeLimit:    DefaultTimeLimit,
					Filter:       DefaultFilter,
					PageSize:     10,
				},
				QueryAttribute: DefaultQueryAttribute,
			},

			attributeValue: "bar",
			attributes:     DefaultAttributes,
			expectedRequest: &ldap.SearchRequest{
				BaseDN:       DefaultBaseDN,
				Scope:        int(DefaultScope),
				DerefAliases: int(DefaultDerefAliases),
				SizeLimit:    DefaultSizeLimit,
				TimeLimit:    DefaultTimeLimit,
				TypesOnly:    DefaultTypesOnly,
				Filter:       fmt.Sprintf("(&(%s)(%s=%s))", DefaultFilter, DefaultQueryAttribute, "bar"),
				Attributes:   DefaultAttributes,
				Controls:     []ldap.Control{ldap.NewControlPaging(10)},
			},
			expectedError: false,
		},
	}

	for _, testCase := range testCases {
		request, err := testCase.options.NewSearchRequest(
			testCase.attributeValue,
			testCase.attributes)

		switch {
		case err != nil && !testCase.expectedError:
			t.Errorf("%s: expected no error but got: %v", testCase.name, err)
		case err == nil && testCase.expectedError:
			t.Errorf("%s: expected an error but got none", testCase.name)
		}

		if !reflect.DeepEqual(testCase.expectedRequest, request) {
			t.Errorf("%s: did not correctly create search request:\n\texpected:\n%#v\n\tgot:\n%#v",
				testCase.name, testCase.expectedRequest, request)
		}
	}
}

// TestErrNoSuchObject tests that our LDAP search correctly wraps the LDAP server error
func TestErrNoSuchObject(t *testing.T) {
	var testCases = []struct {
		name          string
		searchRequest *ldap.SearchRequest
		expectedError error
	}{
		{
			name: "valid search",
			searchRequest: &ldap.SearchRequest{
				BaseDN: "uid=john,o=users,dc=example,dc=com",
			},
			expectedError: nil,
		},
		{
			name: "invalid search",
			searchRequest: &ldap.SearchRequest{
				BaseDN: "ou=groups,dc=example,dc=com",
			},
			expectedError: &errNoSuchObject{baseDN: "ou=groups,dc=example,dc=com"},
		},
	}
	for _, testCase := range testCases {
		testClient := testclient.NewMatchingSearchErrorClient(testclient.New(),
			"ou=groups,dc=example,dc=com",
			ldap.NewError(ldap.LDAPResultNoSuchObject, errors.New("")),
		)
		testConfig := testclient.NewConfig(testClient)
		if _, err := QueryForEntries(testConfig, testCase.searchRequest); !reflect.DeepEqual(err, testCase.expectedError) {
			t.Errorf("%s: error did not match:\n\texpected:\n\t%v\n\tgot:\n\t%v", testCase.name, testCase.expectedError, err)
		}
	}
}

// TestErrEntryNotFound checks that we wrap a zero-length list of results correctly if we search for a unique entry
func TestErrEntryNotFound(t *testing.T) {
	testConfig := testclient.NewConfig(testclient.New())
	testSearchRequest := &ldap.SearchRequest{
		BaseDN:       "dc=example,dc=com",
		Scope:        ldap.ScopeWholeSubtree,
		DerefAliases: int(DefaultDerefAliases),
		SizeLimit:    DefaultSizeLimit,
		TimeLimit:    DefaultTimeLimit,
		TypesOnly:    DefaultTypesOnly,
		Filter:       "(objectClass=*)",
		Attributes:   append(DefaultAttributes),
		Controls:     DefaultControls,
	}

	expectedErr := &errEntryNotFound{baseDN: "dc=example,dc=com", filter: "(objectClass=*)"}

	// test that a unique search errors on no result
	if _, err := QueryForUniqueEntry(testConfig, testSearchRequest); !reflect.DeepEqual(err, expectedErr) {
		t.Errorf("query for unique entry did not get correct error:\n\texpected:\n\t%v\n\tgot:\n\t%v", expectedErr, err)
	}

	// test that a non-unique search doesn't error
	if _, err := QueryForEntries(testConfig, testSearchRequest); !reflect.DeepEqual(err, nil) {
		t.Errorf("query for entries did not get correct error:\n\texpected:\n\t%v\n\tgot:\n\t%v", nil, err)
	}
}

func TestQueryWithPaging(t *testing.T) {
	expectedResult := &ldap.SearchResult{
		Entries: []*ldap.Entry{ldap.NewEntry("cn=paging,ou=paging,dc=paging,dc=com", map[string][]string{"paging": {"true"}})},
	}

	testConfig := testclient.NewConfig(testclient.NewPagingOnlyClient(testclient.New(),
		expectedResult,
	))
	testSearchRequest := &ldap.SearchRequest{
		BaseDN:       "dc=example,dc=com",
		Scope:        ldap.ScopeWholeSubtree,
		DerefAliases: int(DefaultDerefAliases),
		SizeLimit:    DefaultSizeLimit,
		TimeLimit:    DefaultTimeLimit,
		TypesOnly:    DefaultTypesOnly,
		Filter:       "(objectClass=*)",
		Attributes:   append(DefaultAttributes),
		Controls:     []ldap.Control{ldap.NewControlPaging(5)},
	}

	// test that a search request with paging controls gets correctly routed to the SearchWithPaging call
	response, err := QueryForEntries(testConfig, testSearchRequest)
	if err != nil {
		t.Errorf("query with paging control should not create error, but got %v", err)
	}
	if !reflect.DeepEqual(expectedResult.Entries, response) {
		t.Errorf("query with paging did not return correct response: expected %v, got %v", expectedResult.Entries, response)
	}
}
