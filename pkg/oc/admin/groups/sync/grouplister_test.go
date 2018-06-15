package syncgroups

import (
	"errors"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	_ "github.com/openshift/origin/pkg/user/apis/user/install"
	userfakeclient "github.com/openshift/origin/pkg/user/generated/internalclientset/fake"
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
				&userapi.Group{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
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
				&userapi.Group{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPUIDAnnotation: "alpha-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.url annotation`,
		},
		"no uid annotation": {
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPURLAnnotation: "test-host:port"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.uid annotation`,
		},
		"no match: different port": {
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port2",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
				&userapi.Group{ObjectMeta: metav1.ObjectMeta{Name: "beta",
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
				&userapi.Group{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
				&userapi.Group{ObjectMeta: metav1.ObjectMeta{Name: "beta",
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
		fakeClient := userfakeclient.NewSimpleClientset(testCase.startingGroups...)
		lister := NewAllOpenShiftGroupLister(testCase.blacklist, "test-host:port", fakeClient.User().Groups())

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
	listFailClient := userfakeclient.NewSimpleClientset()
	listFailClient.PrependReactor("list", "groups", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fail")
	})

	lister := NewAllOpenShiftGroupLister([]string{}, "test-host:port", listFailClient.User().Groups())
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
				{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
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
				{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPUIDAnnotation: "alpha-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			whitelist:   []string{"alpha"},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.url annotation`,
		},
		"no uid annotation": {
			startingGroups: []*userapi.Group{
				{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPURLAnnotation: "test-host:port"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			whitelist:   []string{"alpha"},
			expectedErr: `group "alpha" marked as having been synced did not have an openshift.io/ldap.uid annotation`,
		},
		"no match: different port": {
			startingGroups: []*userapi.Group{
				{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
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
				{ObjectMeta: metav1.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "test-host:port",
						ldaputil.LDAPUIDAnnotation: "alpha-uid",
					},
					Labels: map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
				{ObjectMeta: metav1.ObjectMeta{Name: "beta",
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
		fakeClient := userfakeclient.NewSimpleClientset()
		fakeClient.PrependReactor("get", "groups", func(action clientgotesting.Action) (bool, runtime.Object, error) {
			groups := map[string]*userapi.Group{}
			for _, group := range testCase.startingGroups {
				groups[group.Name] = group
			}
			if group, exists := groups[action.(clientgotesting.GetAction).GetName()]; exists {
				return true, group, nil
			}
			return false, nil, nil
		})
		lister := NewOpenShiftGroupLister(testCase.whitelist, testCase.blacklist, "test-host:port", fakeClient.User().Groups())

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
	listFailClient := userfakeclient.NewSimpleClientset()
	listFailClient.PrependReactor("get", "groups", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("fail")
	})

	lister := NewOpenShiftGroupLister([]string{"alpha", "beta"}, []string{"beta"}, "", listFailClient.User().Groups())
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
