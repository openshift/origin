package groupdetector

import (
	"errors"
	"reflect"
	"testing"

	"gopkg.in/ldap.v2"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/interfaces"
)

func TestGroupBasedDetectorExists(t *testing.T) {
	var testCases = []struct {
		name           string
		groupGetter    interfaces.LDAPGroupGetter
		expectedErr    error
		expectedExists bool
	}{
		{
			name:           "out of bounds error",
			groupGetter:    &errOutOfBoundsGetterExtractor{},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name:           "entry not found error",
			groupGetter:    &errEntryNotFoundGetterExtractor{},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name:           "no such object error",
			groupGetter:    &errNoSuchObjectGetterExtractor{},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name:           "generic error",
			groupGetter:    &genericErrGetterExtractor{},
			expectedErr:    genericError,
			expectedExists: false,
		},
		{
			name:           "no error, no entries",
			groupGetter:    &puppetGetterExtractor{},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name:           "no error, has entries",
			groupGetter:    &puppetGetterExtractor{returnVal: []*ldap.Entry{dummyEntry}},
			expectedErr:    nil,
			expectedExists: true,
		},
	}

	for _, testCase := range testCases {
		locator := NewGroupBasedDetector(testCase.groupGetter)
		exists, err := locator.Exists("ldapGroupUID")
		if !reflect.DeepEqual(err, testCase.expectedErr) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedErr, err)
		}
		if exists != testCase.expectedExists {
			t.Errorf("%s: incorrect existence check: expected %v, got %v", testCase.name, testCase.expectedExists, exists)
		}
	}
}

func TestMemberBasedDetectorExists(t *testing.T) {
	var testCases = []struct {
		name            string
		memberExtractor interfaces.LDAPMemberExtractor
		expectedErr     error
		expectedExists  bool
	}{
		{
			name:            "out of bounds error",
			memberExtractor: &errOutOfBoundsGetterExtractor{},
			expectedErr:     nil,
			expectedExists:  false,
		},
		{
			name:            "entry not found error",
			memberExtractor: &errEntryNotFoundGetterExtractor{},
			expectedErr:     nil,
			expectedExists:  false,
		},
		{
			name:            "no such object error",
			memberExtractor: &errNoSuchObjectGetterExtractor{},
			expectedErr:     nil,
			expectedExists:  false,
		},
		{
			name:            "generic error",
			memberExtractor: &genericErrGetterExtractor{},
			expectedErr:     genericError,
			expectedExists:  false,
		},
		{
			name:            "no error, no entries",
			memberExtractor: &puppetGetterExtractor{},
			expectedErr:     nil,
			expectedExists:  false,
		},
		{
			name:            "no error, has entries",
			memberExtractor: &puppetGetterExtractor{returnVal: []*ldap.Entry{dummyEntry}},
			expectedErr:     nil,
			expectedExists:  true,
		},
	}

	for _, testCase := range testCases {
		locator := NewMemberBasedDetector(testCase.memberExtractor)
		exists, err := locator.Exists("ldapGroupUID")
		if !reflect.DeepEqual(err, testCase.expectedErr) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedErr, err)
		}
		if exists != testCase.expectedExists {
			t.Errorf("%s: incorrect existence check: expected %v, got %v", testCase.name, testCase.expectedExists, exists)
		}
	}
}

func TestCompoundDetectorExists(t *testing.T) {
	var testCases = []struct {
		name           string
		locators       []interfaces.LDAPGroupDetector
		expectedErr    error
		expectedExists bool
	}{
		{
			name:           "no detectors",
			locators:       []interfaces.LDAPGroupDetector{},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name: "none fail to locate",
			locators: []interfaces.LDAPGroupDetector{
				NewGroupBasedDetector(&puppetGetterExtractor{returnVal: []*ldap.Entry{dummyEntry}}),
				NewMemberBasedDetector(&puppetGetterExtractor{returnVal: []*ldap.Entry{dummyEntry}}),
			},
			expectedErr:    nil,
			expectedExists: true,
		},
		{
			name: "one fails to locate",
			locators: []interfaces.LDAPGroupDetector{
				NewGroupBasedDetector(&puppetGetterExtractor{}),
				NewMemberBasedDetector(&puppetGetterExtractor{returnVal: []*ldap.Entry{dummyEntry}}),
			},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name: "all fail to locate because of errors",
			locators: []interfaces.LDAPGroupDetector{
				NewGroupBasedDetector(&errEntryNotFoundGetterExtractor{}),
				NewMemberBasedDetector(&errOutOfBoundsGetterExtractor{}),
			},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name: "all locate no entries",
			locators: []interfaces.LDAPGroupDetector{
				NewGroupBasedDetector(&puppetGetterExtractor{}),
				NewMemberBasedDetector(&puppetGetterExtractor{}),
			},
			expectedErr:    nil,
			expectedExists: false,
		},
		{
			name: "one errors",
			locators: []interfaces.LDAPGroupDetector{
				NewGroupBasedDetector(&puppetGetterExtractor{}),
				NewMemberBasedDetector(&genericErrGetterExtractor{}),
			},
			expectedErr:    genericError,
			expectedExists: false,
		},
	}

	for _, testCase := range testCases {
		locator := NewCompoundDetector(testCase.locators...)
		exists, err := locator.Exists("ldapGroupUID")
		if !reflect.DeepEqual(err, testCase.expectedErr) {
			t.Errorf("%s: incorrect error returned:\n\texpected:\n\t%v\n\tgot:\n\t%v\n", testCase.name, testCase.expectedErr, err)
		}
		if exists != testCase.expectedExists {
			t.Errorf("%s: incorrect existence check: expected %v, got %v", testCase.name, testCase.expectedExists, exists)
		}
	}
}

var dummyEntry *ldap.Entry = &ldap.Entry{DN: "dn"}

// puppetGetterExtractor is a GroupGetter and a MemberExtractor that generates no errors and returns
// whatever LDAP entries are given to it
type puppetGetterExtractor struct {
	returnVal []*ldap.Entry
}

func (g *puppetGetterExtractor) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
	if len(g.returnVal) > 0 {
		return g.returnVal[0], nil
	}
	return nil, nil
}

func (g *puppetGetterExtractor) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	if len(g.returnVal) > 0 {
		return g.returnVal, nil
	}
	return nil, nil
}

// the following generators are both GroupGetters and MemberExtractors that generate specific errors
// whenever their corresponding methods are called. They are used for the tests for this package

var outOfBoundsError error = ldaputil.NewQueryOutOfBoundsError("baseDN", "queryDN")
var entryNotFoundError error = ldaputil.NewEntryNotFoundError("baseDN", "filter")
var noSuchObjectError error = ldaputil.NewNoSuchObjectError("baseDN")
var genericError error = errors.New("generic error")

type errOutOfBoundsGetterExtractor struct{}

func (g *errOutOfBoundsGetterExtractor) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
	return nil, outOfBoundsError
}

func (g *errOutOfBoundsGetterExtractor) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	return nil, outOfBoundsError
}

type errEntryNotFoundGetterExtractor struct{}

func (g *errEntryNotFoundGetterExtractor) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
	return nil, entryNotFoundError
}

func (g *errEntryNotFoundGetterExtractor) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	return nil, entryNotFoundError
}

type errNoSuchObjectGetterExtractor struct{}

func (g *errNoSuchObjectGetterExtractor) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
	return nil, noSuchObjectError
}

func (g *errNoSuchObjectGetterExtractor) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	return nil, noSuchObjectError
}

type genericErrGetterExtractor struct{}

func (g *genericErrGetterExtractor) GroupEntryFor(ldapGroupUID string) (*ldap.Entry, error) {
	return nil, genericError
}

func (g *genericErrGetterExtractor) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	return nil, genericError
}
