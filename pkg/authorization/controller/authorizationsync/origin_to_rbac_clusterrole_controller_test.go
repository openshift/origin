package authorizationsync

import (
	"fmt"
	"strings"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	rbaclister "k8s.io/kubernetes/pkg/client/listers/rbac/internalversion"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	originlister "github.com/openshift/origin/pkg/authorization/generated/listers/authorization/internalversion"
)

func TestSyncClusterRole(t *testing.T) {
	tests := []struct {
		name string

		key            string
		startingRBAC   []*rbac.ClusterRole
		startingOrigin []*authorizationapi.ClusterRole
		reactions      map[reactionMatch]clienttesting.ReactionFunc

		actionCheck   func([]clienttesting.Action) error
		expectedError string
	}{
		{
			name: "no action on missing both",
			key:  "resource-01",
			actionCheck: func(actions []clienttesting.Action) error {
				if len(actions) != 0 {
					return fmt.Errorf("expected %v, got %v", 0, actions)
				}
				return nil
			},
		},
		{
			name: "simple create",
			key:  "resource-01",
			startingOrigin: []*authorizationapi.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01"}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				action, err := ensureSingleCreateAction(actions)
				if err != nil {
					return err
				}
				if e, a := "resource-01", action.GetObject().(*rbac.ClusterRole).Name; e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				return nil
			},
		},
		{
			name: "simple create with normalization",
			key:  "resource-01",
			startingOrigin: []*authorizationapi.ClusterRole{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "resource-01"},
					Rules: []authorizationapi.PolicyRule{
						{
							Verbs:     sets.NewString("CREATE"),
							Resources: sets.NewString("NAMESPACE"),
							APIGroups: []string{"V2"},
						},
					},
				},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				action, err := ensureSingleCreateAction(actions)
				if err != nil {
					return err
				}
				rbacRole := action.GetObject().(*rbac.ClusterRole)
				if e, a := "resource-01", rbacRole.Name; e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				expectedRBACRules := []rbac.PolicyRule{
					{
						Verbs:     []string{"create"},
						Resources: []string{"namespace"},
						APIGroups: []string{"v2"},
					},
				}
				if !apiequality.Semantic.DeepEqual(expectedRBACRules, rbacRole.Rules) {
					return fmt.Errorf("expected %v, got %v", expectedRBACRules, rbacRole.Rules)
				}
				return nil
			},
		},
		{
			name: "delete on missing origin",
			key:  "resource-01",
			startingRBAC: []*rbac.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01"}},
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
			key:  "resource-01",
			startingRBAC: []*rbac.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01"}},
			},
			startingOrigin: []*authorizationapi.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01", Annotations: map[string]string{"foo": "different"}}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				action, err := ensureSingleUpdateAction(actions)
				if err != nil {
					return err
				}
				if e, a := "resource-01", action.GetObject().(*rbac.ClusterRole).Name; e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				return nil
			},
		},
		{
			name: "update with annotation transform",
			key:  "resource-01",
			startingRBAC: []*rbac.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01"}},
			},
			startingOrigin: []*authorizationapi.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01", Annotations: map[string]string{"openshift.io/reconcile-protect": "true"}}},
			},
			actionCheck: func(actions []clienttesting.Action) error {
				action, err := ensureSingleUpdateAction(actions)
				if err != nil {
					return err
				}
				if e, a := "resource-01", action.GetObject().(*rbac.ClusterRole).Name; e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				if e, a := "false", action.GetObject().(*rbac.ClusterRole).Annotations["rbac.authorization.kubernetes.io/autoupdate"]; e != a {
					return fmt.Errorf("expected %v, got %v", e, a)
				}
				return nil
			},
		},
		{
			name: "no action on zero diff",
			key:  "resource-01",
			startingRBAC: []*rbac.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01"}},
			},
			startingOrigin: []*authorizationapi.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01"}},
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
			key:  "resource-01",
			startingRBAC: []*rbac.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01"}},
			},
			startingOrigin: []*authorizationapi.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "resource-01", Annotations: map[string]string{"foo": "different"}}},
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
				{verb: "update", resource: "clusterroles"}: func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, apierrors.NewInvalid(rbac.Kind("ClusterRole"), "dummy", nil)
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

		c := &OriginClusterRoleToRBACClusterRoleController{
			rbacClient:   fakeClient.Rbac(),
			rbacLister:   rbaclister.NewClusterRoleLister(rbacIndexer),
			originLister: originlister.NewClusterRoleLister(originIndexer),
		}
		err := c.syncClusterRole(tc.key)
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
