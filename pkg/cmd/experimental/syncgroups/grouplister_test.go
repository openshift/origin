package syncgroups

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/client/testclient"
	userapi "github.com/openshift/origin/pkg/user/api"
)

func TestListGroups(t *testing.T) {
	testCases := map[string]struct {
		hostURL        string
		startingGroups []runtime.Object
		expectedName   string
		expectedErr    string
	}{
		"good": {
			hostURL: "test-host:port",
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPURLAnnotation: "test-host:port", ldaputil.LDAPUIDAnnotation: "alpha-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedName: "alpha-uid",
		},
		"no annotation": {
			hostURL: "test-host:port",
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPUIDAnnotation: "alpha-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedErr: `group "alpha": openshift.io/ldap.url annotation expected: wanted test-host:port`,
		},
		"different port": {
			hostURL: "test-host:port",
			startingGroups: []runtime.Object{
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "alpha",
					Annotations: map[string]string{ldaputil.LDAPURLAnnotation: "test-host:port2", ldaputil.LDAPUIDAnnotation: "alpha-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
				&userapi.Group{ObjectMeta: kapi.ObjectMeta{Name: "beta",
					Annotations: map[string]string{ldaputil.LDAPURLAnnotation: "test-host:port", ldaputil.LDAPUIDAnnotation: "beta-uid"},
					Labels:      map[string]string{ldaputil.LDAPHostLabel: "test-host"}}},
			},
			expectedName: "beta-uid",
		},
	}

	for name, testCase := range testCases {
		fakeClient := testclient.NewSimpleFake(testCase.startingGroups...)
		lister := NewAllOpenShiftGroupLister(testCase.hostURL, fakeClient.Groups())

		groupNames, err := lister.ListGroups()
		if err != nil && len(testCase.expectedErr) == 0 {
			t.Errorf("%s: unexpected error: %v", name, err)
		} else if err == nil && len(testCase.expectedErr) != 0 {
			t.Errorf("%s: expected %v, got nil", name, testCase.expectedErr)
		} else if err != nil {
			if expected, actual := testCase.expectedErr, err.Error(); expected != actual {
				t.Errorf("%s: expected %v, got %v", name, expected, actual)
			}
		}

		if len(groupNames) == 1 && groupNames[0] != testCase.expectedName {
			t.Errorf("%s: expected %s, got %s", name, testCase.expectedName, groupNames[0])
		}
	}
}
