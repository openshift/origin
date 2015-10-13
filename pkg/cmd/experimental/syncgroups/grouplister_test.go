package syncgroups

import (
	"errors"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/client/testclient"
	userapi "github.com/openshift/origin/pkg/user/api"
)

func TestListAllOpenShiftGroups(t *testing.T) {
	testCases := map[string]struct {
		startingGroups []runtime.Object
		blacklist      []string
		expectedName   string
		expectedErr    string
	}{
		"good": {
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedName: "alpha-uid",
		},
		"no url annotation": {
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPUIDAnnotation: "alpha-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.url annotation`,
		},
		"no uid annotation": {
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPURLAnnotation: "test-host:port"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.uid annotation`,
		},
		"no match: different port": {
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port2",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "beta",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "beta-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedName: "beta-uid",
		},
		"blacklist": {
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "beta",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "beta-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			blacklist:    []string{"alpha"},
			expectedName: "beta-uid",
		},
	}

	for name, testCase := range testCases {
		fakeClient := testclient.NewSimpleFake(testCase.startingGroups...)
		lister := NewAllOpenShiftGroupLister(testCase.blacklist, "test-host:port", fakeClient.Groups())

		groupNames, err := lister.ListGroups()
		if err != nil {
			if len(testCase.expectedErr) == 0 {
				t.Errorf("%s: unexpected error: %v", name, err)
			}
			if expected, actual := testCase.expectedErr, err.Error(); expected != actual {
				t.Errorf("%s: expected error %v, got %v", name, expected, actual)
			}
		} else {
			if len(testCase.expectedErr) != 0 {
				t.Errorf("%s: expected error %v, got nil", name, testCase.expectedErr)
			}
			if expected, actual := []string{testCase.expectedName}, groupNames; !reflect.DeepEqual(expected, actual) {
				t.Errorf("%s: expected UIDs %v, got %v", name, expected, actual)
			}
		}
	}
}

func TestListAllOpenShiftGroupsListErr(t *testing.T) {
	listFailClient := testclient.NewSimpleFake()
	listFailClient.PrependReactor("list", "groups", func(action ktestclient.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fail")
	})

	lister := NewAllOpenShiftGroupLister([]string{}, "test-host:port", listFailClient.Groups())
	groupUIDs, err := lister.ListGroups()
	if err == nil {
		t.Error("expected an error listing groups, got none")
		if len(groupUIDs) != 0 {
			t.Errorf("did not expect any groups to be listed, got: %v", groupUIDs)
		}
	} else {
		if expected, actual := "fail", err.Error(); expected != actual {
			t.Errorf("did not get correct error listing groups: expected %q, got %q", expected, actual)
		}
	}
}

func TestListWhitelistOpenShiftGroups(t *testing.T) {
	testCases := map[string]struct {
		startingGroups []*userapi.Group
		whitelist      []string
		blacklist      []string
		expectedName   string
		expectedErr    string
	}{
		"good": {
			startingGroups: []*userapi.Group{
				{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			whitelist:    []string{"alpha"},
			expectedName: "alpha-uid",
		},
		"no url annotation": {
			startingGroups: []*userapi.Group{
				{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPUIDAnnotation: "alpha-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			whitelist:   []string{"alpha"},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.url annotation`,
		},
		"no uid annotation": {
			startingGroups: []*userapi.Group{
				{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPURLAnnotation: "test-host:port"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			whitelist:   []string{"alpha"},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.uid annotation`,
		},
		"no match: different port": {
			startingGroups: []*userapi.Group{
				{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port2",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			whitelist:   []string{"alpha"},
			expectedErr: `group "alpha" was not synchronized from: test-host:port`,
		},
		"blacklist": {
			startingGroups: []*userapi.Group{
				{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
				{ObjectMeta: kapi.ObjectMeta{Name: "beta",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "beta-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			whitelist:    []string{"alpha", "beta"},
			blacklist:    []string{"alpha"},
			expectedName: "beta-uid",
		},
	}

	for name, testCase := range testCases {
		fakeClient := testclient.NewSimpleFake()
		fakeClient.PrependReactor("get", "groups", func(action ktestclient.Action) (bool, runtime.Object, error) {
			groups := map[string]*userapi.Group{}
			for _, group := range testCase.startingGroups {
				groups[group.Name] = group
			}
			if group, exists := groups[action.(ktestclient.GetAction).GetName()]; exists {
				return true, group, nil
			}
			return false, nil, nil
		})
		lister := NewOpenShiftGroupLister(testCase.whitelist, testCase.blacklist, "test-host:port", fakeClient.Groups())

		groupNames, err := lister.ListGroups()
		if err != nil {
			if len(testCase.expectedErr) == 0 {
				t.Errorf("%s: unexpected error: %v", name, err)
			}
			if expected, actual := testCase.expectedErr, err.Error(); expected != actual {
				t.Errorf("%s: expected error %v, got %v", name, expected, actual)
			}
		} else {
			if len(testCase.expectedErr) != 0 {
				t.Errorf("%s: expected error %v, got nil", name, testCase.expectedErr)
			}
			if expected, actual := []string{testCase.expectedName}, groupNames; !reflect.DeepEqual(expected, actual) {
				t.Errorf("%s: expected UIDs %v, got %v", name, expected, actual)
			}
		}
	}
}

func TestListOpenShiftGroupsListErr(t *testing.T) {
	listFailClient := testclient.NewSimpleFake()
	listFailClient.PrependReactor("get", "groups", func(action ktestclient.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fail")
	})

	lister := NewOpenShiftGroupLister([]string{"alpha", "beta"}, []string{"beta"}, "", listFailClient.Groups())
	groupUIDs, err := lister.ListGroups()
	if err == nil {
		t.Error("expected an error listing groups, got none")
		if len(groupUIDs) != 0 {
			t.Errorf("did not expect any groups to be listed, got: %v", groupUIDs)
		}
	} else {
		if expected, actual := "fail", err.Error(); expected != actual {
			t.Errorf("did not get correct error listing groups: expected %q, got %q", expected, actual)
		}
	}
}

func TestLDAPBlacklistFilter(t *testing.T) {
	whitelister := NewLDAPWhitelistGroupLister([]string{"rebecca", "valerie"})
	blacklister := NewLDAPBlacklistGroupLister([]string{"rebecca"}, whitelister)

	result, err := blacklister.ListGroups()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := []string{"valerie"}, result; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
}
