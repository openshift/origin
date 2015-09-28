package ldaputil

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/go-ldap/ldap"
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
	}

	for _, testCase := range testCases {
		request, err := testCase.options.NewSearchRequest(
			testCase.attributeValue,
			testCase.attributes)

		switch {
		case err != nil && !testCase.expectedError:
			t.Errorf("%s: expected no error but got: %v", testCase.name, err)
		case err == nil && testCase.expectedError:
			t.Error("%s: expected an error but got none")
		}

		if !reflect.DeepEqual(testCase.expectedRequest, request) {
			t.Errorf("%s: did not correctly create search request:\n\texpected:\n%#v\n\tgot:\n%#v",
				testCase.name, testCase.expectedRequest, request)
		}
	}
}
