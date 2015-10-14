package syncgroups

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/client/testclient"
	userapi "github.com/openshift/origin/pkg/user/api"
)

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

func TestAllOpenShiftFilter(t *testing.T) {
	tc := testclient.NewSimpleFake()
	tc.PrependReactor("list", "groups", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		groupList := &userapi.GroupList{}

		groupA := userapi.Group{
			ObjectMeta: kapi.ObjectMeta{
				Name: "groupA",
				Annotations: map[string]string{
					ldaputil.LDAPURLAnnotation: "host:port",
					ldaputil.LDAPUIDAnnotation: "ldapGroupA",
				},
			},
		}
		groupB := userapi.Group{
			ObjectMeta: kapi.ObjectMeta{
				Name: "groupB",
				Annotations: map[string]string{
					ldaputil.LDAPURLAnnotation: "host:port",
					ldaputil.LDAPUIDAnnotation: "ldapGroupB",
				},
			},
		}
		groupList.Items = append(groupList.Items, groupA, groupB)

		return true, groupList, nil
	})

	lister := NewAllOpenShiftGroupLister("host:port", tc.Groups(), []string{"groupA"})

	ldapGroups, err := lister.ListGroups()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := []string{"ldapGroupB"}, ldapGroups; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestOpenShiftWhitelistFilter(t *testing.T) {
	tc := testclient.NewSimpleFake()
	tc.PrependReactor("get", "groups", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		switch action.(ktestclient.GetAction).GetName() {
		case "groupA":
			return true, &userapi.Group{
				ObjectMeta: kapi.ObjectMeta{
					Name: "groupA",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "host:port",
						ldaputil.LDAPUIDAnnotation: "ldapGroupA",
					},
				},
			}, nil
		case "groupB":
			return true, &userapi.Group{
				ObjectMeta: kapi.ObjectMeta{
					Name: "groupB",
					Annotations: map[string]string{
						ldaputil.LDAPURLAnnotation: "host:port",
						ldaputil.LDAPUIDAnnotation: "ldapGroupB",
					},
				},
			}, nil
		}

		return false, nil, nil
	})

	lister := NewOpenShiftGroupLister([]string{"groupA", "groupB"}, []string{"groupA"}, "host:port", tc.Groups())

	ldapGroups, err := lister.ListGroups()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if e, a := []string{"ldapGroupB"}, ldapGroups; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
}
