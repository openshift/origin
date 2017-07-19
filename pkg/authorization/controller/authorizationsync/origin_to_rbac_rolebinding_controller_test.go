package authorizationsync

import (
	"fmt"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	rbaclister "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	originlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
)

func TestSyncRoleBinding(t *testing.T) {
	tests := []struct {
		name string

		key            string
		startingRBAC   []*rbac.RoleBinding
		startingOrigin []*authorizationapi.RoleBinding
		reactions      map[reactionMatch]clienttesting.ReactionFunc

		actionCheck   func([]clienttesting.Action) error
		expectedError string
	}{
		{
			name: "no action on missing both",
			key:  "foo/resource-01",
			actionCheck: func(actions []clienttesting.Action) error {
				if len(actions) != 0 {
					return fmt.Errorf("expected %v, got %v", 0, actions)
				}
				return nil
			},
		},
		{
			name: "simple create",
			key:  "foo/resource-01",
			startingOrigin: []*authorizationapi.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01"}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				action, err := ensureSingleCreateAction(actions)
				if err != nil {
					return err
				}
				if e, a := "resource-01", action.GetObject().(*rbac.RoleBinding).Name; e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				return nil
			},
		},
		{
			name: "delete on missing origin",
			key:  "foo/resource-01",
			startingRBAC: []*rbac.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01"}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				action, err := ensureSingleDeleteAction(actions)
				if err != nil {
					return err
				}
				if e, a := "resource-01", action.GetName(); e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				return nil
			},
		},
		{
			name: "simple update",
			key:  "foo/resource-01",
			startingRBAC: []*rbac.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01"}},
			},
			startingOrigin: []*authorizationapi.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01", Annotations: map[string]string{"foo": "different"}}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				action, err := ensureSingleUpdateAction(actions)
				if err != nil {
					return err
				}
				if e, a := "resource-01", action.GetObject().(*rbac.RoleBinding).Name; e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				return nil
			},
		},
		{
			name: "no action on zero diff",
			key:  "foo/resource-01",
			startingRBAC: []*rbac.RoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01"},
					RoleRef:    rbac.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
				},
			},
			startingOrigin: []*authorizationapi.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01"}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				if len(actions) != 0 {
					return fmt.Errorf("expected %v, got %v", 0, actions)
				}
				return nil
			},
		},
		{
			name: "invalid update",
			key:  "foo/resource-01",
			startingRBAC: []*rbac.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01"}},
			},
			startingOrigin: []*authorizationapi.RoleBinding{
				{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "resource-01", Annotations: map[string]string{"foo": "different"}}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				if len(actions) != 2 {
					return fmt.Errorf("expected update then delete, got %v", actions)
				}
				if _, ok := actions[0].(clienttesting.UpdateAction); !ok {
					return fmt.Errorf("expected update, got %v", actions)
				}
				if _, ok := actions[1].(clienttesting.DeleteAction); !ok {
					return fmt.Errorf("expected delete, got %v", actions)
				}
				return nil
			},
			reactions: map[reactionMatch]clienttesting.ReactionFunc{
				{verb: "update", resource: "rolebindings"}: func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewInvalid(rbac.Kind("RoleBinding"), "dummy", nil)
				},
			},
			expectedError: "is invalid",
		},
	}

	for _, tc := range tests {
		objs := []runtime.Object{}
		rbacIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		originIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		for _, obj := range tc.startingRBAC {
			rbacIndexer.Add(obj)
			objs = append(objs, obj)
		}
		for _, obj := range tc.startingOrigin {
			originIndexer.Add(obj)
		}
		fakeClient := fake.NewSimpleClientset(objs...)
		for reactionMatch, action := range tc.reactions {
			fakeClient.PrependReactor(reactionMatch.verb, reactionMatch.resource, action)
		}

		c := &OriginRoleBindingToRBACRoleBindingController{
			rbacClient:   fakeClient.Rbac(),
			rbacLister:   rbaclister.NewRoleBindingLister(rbacIndexer),
			originLister: originlister.NewRoleBindingLister(originIndexer),
		}
		err := c.syncRoleBinding(tc.key)
		switch {
		case len(tc.expectedError) == 0 && err == nil:
		case len(tc.expectedError) == 0 && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case len(tc.expectedError) != 0 && err == nil:
			t.Errorf("%s: missing %v", tc.name, tc.expectedError)
		case len(tc.expectedError) != 0 && err != nil && !strings.Contains(err.Error(), tc.expectedError):
			t.Errorf("%s: expected %v, got %v", tc.name, tc.expectedError, err)
		}

		if err := tc.actionCheck(fakeClient.Actions()); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}
