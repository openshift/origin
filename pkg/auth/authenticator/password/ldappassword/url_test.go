package ldappassword

import (
	"reflect"
	"testing"

	"github.com/go-ldap/ldap"
)

func TestParseURL(t *testing.T) {
	testcases := map[string]struct {
		URL             string
		ExpectedLDAPURL LDAPURL
		ExpectedError   string
	}{
		// Defaults
		"defaults for ldap://": {
			URL:             "ldap://",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldap", Host: "localhost:389", BaseDN: "", QueryAttribute: "uid", Scope: ldap.ScopeWholeSubtree, Filter: "(objectClass=*)"},
		},
		"defaults for ldaps://": {
			URL:             "ldaps://",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldaps", Host: "localhost:636", BaseDN: "", QueryAttribute: "uid", Scope: ldap.ScopeWholeSubtree, Filter: "(objectClass=*)"},
		},

		// Valid
		"fully specified": {
			URL:             "ldap://myhost:123/o=myorg?cn?one?(o=mygroup*)?ext=1",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldap", Host: "myhost:123", BaseDN: "o=myorg", QueryAttribute: "cn", Scope: ldap.ScopeSingleLevel, Filter: "(o=mygroup*)"},
		},
		"first attribute used for query": {
			URL:             "ldap://myhost:123/o=myorg?cn,uid?one?(o=mygroup*)?ext=1",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldap", Host: "myhost:123", BaseDN: "o=myorg", QueryAttribute: "cn", Scope: ldap.ScopeSingleLevel, Filter: "(o=mygroup*)"},
		},

		// Escaping
		"percent escaped 1": {
			URL:             "ldap://myhost:123/o=my%20org?my%20attr?one?(o=my%20group%3f*)?ext=1",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldap", Host: "myhost:123", BaseDN: "o=my org", QueryAttribute: "my attr", Scope: ldap.ScopeSingleLevel, Filter: "(o=my group?*)"},
		},
		"percent escaped 2": {
			URL:             "ldap://myhost:123/o=Babsco,c=US???(four-octet=%5c00%5c00%5c00%5c04)",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldap", Host: "myhost:123", BaseDN: "o=Babsco,c=US", QueryAttribute: "uid", Scope: ldap.ScopeWholeSubtree, Filter: `(four-octet=\00\00\00\04)`},
		},
		"percent escaped 3": {
			URL:             "ldap://myhost:123/o=An%20Example%5C2C%20Inc.,c=US",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldap", Host: "myhost:123", BaseDN: `o=An Example\2C Inc.,c=US`, QueryAttribute: "uid", Scope: ldap.ScopeWholeSubtree, Filter: "(objectClass=*)"},
		},

		// Invalid
		"empty": {
			URL:           "",
			ExpectedError: `invalid scheme ""`,
		},
		"invalid scheme": {
			URL:           "http://myhost:123/o=myorg?cn?one?(o=mygroup*)?ext=1",
			ExpectedError: `invalid scheme "http"`,
		},
		"invalid scope": {
			URL:           "ldap://myhost:123/o=myorg?cn?foo?(o=mygroup*)?ext=1",
			ExpectedError: `invalid scope "foo"`,
		},
		"invalid filter": {
			URL:           "ldap://myhost:123/o=myorg?cn?one?(mygroup*)?ext=1",
			ExpectedError: `invalid filter: LDAP Result Code 201 "": ldap: error parsing filter`,
		},
		"invalid segments": {
			URL:           "ldap://myhost:123/o=myorg?cn?one?(o=mygroup*)?ext=1?extrasegment",
			ExpectedError: `too many query options "cn?one?(o=mygroup*)?ext=1?extrasegment"`,
		},

		// Extension handling
		"ignored optional extension": {
			URL:             "ldap:///??sub??e-bindname=cn=Manager%2cdc=example%2cdc=com",
			ExpectedLDAPURL: LDAPURL{Scheme: "ldap", Host: "localhost:389", BaseDN: "", QueryAttribute: "uid", Scope: ldap.ScopeWholeSubtree, Filter: "(objectClass=*)"},
		},
		"rejected required extension": {
			URL:           "ldap:///??sub??!e-bindname=cn=Manager%2cdc=example%2cdc=com",
			ExpectedError: "unsupported critical extension !e-bindname=cn=Manager%2cdc=example%2cdc=com",
		},
	}

	for k, tc := range testcases {
		ldapURL, err := ParseURL(tc.URL)
		if err != nil {
			if len(tc.ExpectedError) == 0 {
				t.Errorf("%s: Unexpected error: %v", k, err)
			}
			if err.Error() != tc.ExpectedError {
				t.Errorf("%s: Expected error %q, got %v", k, tc.ExpectedError, err)
			}
			continue
		}
		if len(tc.ExpectedError) > 0 {
			t.Errorf("%s: Expected error %q, got none", k, tc.ExpectedError)
			continue
		}
		if !reflect.DeepEqual(tc.ExpectedLDAPURL, ldapURL) {
			t.Errorf("%s: Expected\n\t%#v\ngot\n\t%#v", k, tc.ExpectedLDAPURL, ldapURL)
			continue
		}
	}
}
