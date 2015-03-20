package cache

import (
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	testregistry "github.com/openshift/origin/pkg/authorization/registry/test"
)

func TestPolicyGet(t *testing.T) {
	policyStop := make(chan struct{})
	bindingStop := make(chan struct{})
	defer close(policyStop)
	defer close(bindingStop)

	policyRegistry := testregistry.NewPolicyRegistry(testPolicies(), nil)
	bindingRegistry := testregistry.NewPolicyBindingRegistry(testBindings(), nil)

	policyCache := NewPolicyCache(bindingRegistry, policyRegistry)
	policyCache.RunUntil(bindingStop, policyStop)

	testStop := make(chan struct{})

	util.Until(func() {
		ctx := kapi.WithNamespace(kapi.NewContext(), "mallet")
		policy, policyErr := policyCache.GetPolicy(ctx, authorizationapi.PolicyName)

		bindings, bindingErr := policyCache.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
		if (policyErr == nil) && (bindingErr == nil) && (policy != nil) && (len(bindings.Items) == 1) {
			close(testStop)
		}

	}, 1*time.Millisecond, testStop)
}

func testPolicies() []authorizationapi.Policy {
	return []authorizationapi.Policy{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      authorizationapi.PolicyName,
				Namespace: "mallet",
			},
			Roles: map[string]authorizationapi.Role{},
		}}
}
func testBindings() []authorizationapi.PolicyBinding {
	return []authorizationapi.PolicyBinding{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name:      "mallet",
				Namespace: "mallet",
			},
			RoleBindings: map[string]authorizationapi.RoleBinding{
				"projectAdmins": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "projectAdmins",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name:      "admin",
						Namespace: "mallet",
					},
					Users: util.NewStringSet("Matthew"),
				},
				"viewers": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "viewers",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name:      "view",
						Namespace: "mallet",
					},
					Users: util.NewStringSet("Victor"),
				},
				"editors": {
					ObjectMeta: kapi.ObjectMeta{
						Name:      "editors",
						Namespace: "mallet",
					},
					RoleRef: kapi.ObjectReference{
						Name:      "edit",
						Namespace: "mallet",
					},
					Users: util.NewStringSet("Edgar"),
				},
			},
		},
	}
}
