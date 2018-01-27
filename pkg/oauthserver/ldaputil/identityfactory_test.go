package ldaputil

import (
	"testing"

	"gopkg.in/ldap.v2"
)

func TestGetAttributeValue(t *testing.T) {

	testcases := map[string]struct {
		Entry         *ldap.Entry
		Attributes    []string
		ExpectedValue string
	}{
		"empty": {
			Attributes:    []string{},
			Entry:         &ldap.Entry{DN: "", Attributes: []*ldap.EntryAttribute{}},
			ExpectedValue: "",
		},

		"dn": {
			Attributes:    []string{"dn"},
			Entry:         &ldap.Entry{DN: "foo", Attributes: []*ldap.EntryAttribute{}},
			ExpectedValue: "foo",
		},
		"DN": {
			Attributes:    []string{"DN"},
			Entry:         &ldap.Entry{DN: "foo", Attributes: []*ldap.EntryAttribute{}},
			ExpectedValue: "foo",
		},

		"missing": {
			Attributes:    []string{"foo", "bar", "baz"},
			Entry:         &ldap.Entry{DN: "", Attributes: []*ldap.EntryAttribute{}},
			ExpectedValue: "",
		},

		"present": {
			Attributes: []string{"foo"},
			Entry: &ldap.Entry{DN: "", Attributes: []*ldap.EntryAttribute{
				{Name: "foo", Values: []string{"fooValue"}},
			}},
			ExpectedValue: "fooValue",
		},
		"first of multi-value attribute": {
			Attributes: []string{"foo"},
			Entry: &ldap.Entry{DN: "", Attributes: []*ldap.EntryAttribute{
				{Name: "foo", Values: []string{"fooValue", "fooValue2"}},
			}},
			ExpectedValue: "fooValue",
		},
		"first present attribute": {
			Attributes: []string{"foo", "bar", "baz"},
			Entry: &ldap.Entry{DN: "", Attributes: []*ldap.EntryAttribute{
				{Name: "foo", Values: []string{""}},
				{Name: "bar", Values: []string{"barValue"}},
				{Name: "baz", Values: []string{"bazValue"}},
			}},
			ExpectedValue: "barValue",
		},
	}

	for k, tc := range testcases {
		v := GetAttributeValue(tc.Entry, tc.Attributes)
		if v != tc.ExpectedValue {
			t.Errorf("%s: Expected %q, got %q", k, tc.ExpectedValue, v)
		}
	}

}
