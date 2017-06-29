package util

import (
	"errors"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/kubernetes/pkg/apis/authorization"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"

	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicyetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	clusterpolicybindingetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrole"
	clusterrolestorage "github.com/openshift/origin/pkg/authorization/registry/clusterrole/proxy"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding"
	clusterrolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding/proxy"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	"github.com/openshift/origin/pkg/authorization/registry/role"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	"github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// AddUserToSAR adds the requisite user information to a SubjectAccessReview.
// It returns the modified SubjectAccessReview.
func AddUserToSAR(user user.Info, sar *authorization.SubjectAccessReview) *authorization.SubjectAccessReview {
	sar.Spec.User = user.GetName()
	// reminiscent of the bad old days of C.  Copies copy the min number of elements of both source and dest
	sar.Spec.Groups = make([]string, len(user.GetGroups()), len(user.GetGroups()))
	copy(sar.Spec.Groups, user.GetGroups())
	sar.Spec.Extra = map[string]authorization.ExtraValue{}

	for k, v := range user.GetExtra() {
		sar.Spec.Extra[k] = authorization.ExtraValue(v)
	}

	return sar
}

// Authorize verifies that a given user is permitted to carry out a given
// action.  If this cannot be determined, or if the user is not permitted, an
// error is returned.
func Authorize(sarClient internalversion.SubjectAccessReviewInterface, user user.Info, resourceAttributes *authorization.ResourceAttributes) error {
	sar := AddUserToSAR(user, &authorization.SubjectAccessReview{
		Spec: authorization.SubjectAccessReviewSpec{
			ResourceAttributes: resourceAttributes,
		},
	})

	resp, err := sarClient.Create(sar)
	if err == nil && resp != nil && resp.Status.Allowed {
		return nil
	}

	if err == nil {
		err = errors.New(resp.Status.Reason)
	}
	return kerrors.NewForbidden(schema.GroupResource{Group: resourceAttributes.Group, Resource: resourceAttributes.Resource}, resourceAttributes.Name, err)
}

func NewLiveRuleResolver(policyRegistry policyregistry.Registry, policyBindingRegistry policybindingregistry.Registry, clusterPolicyRegistry clusterpolicyregistry.Registry, clusterBindingRegistry clusterpolicybindingregistry.Registry) rulevalidation.AuthorizationRuleResolver {
	return rulevalidation.NewDefaultRuleResolver(
		&policyregistry.ReadOnlyPolicyListerNamespacer{
			Registry: policyRegistry,
		},
		&policybindingregistry.ReadOnlyPolicyBindingListerNamespacer{
			Registry: policyBindingRegistry,
		},
		&clusterpolicyregistry.ReadOnlyClusterPolicyClientShim{
			ReadOnlyClusterPolicy: clusterpolicyregistry.ReadOnlyClusterPolicy{Registry: clusterPolicyRegistry},
		},
		&clusterpolicybindingregistry.ReadOnlyClusterPolicyBindingClientShim{
			ReadOnlyClusterPolicyBinding: clusterpolicybindingregistry.ReadOnlyClusterPolicyBinding{Registry: clusterBindingRegistry},
		},
	)
}

func GetAuthorizationStorage(optsGetter restoptions.Getter, cachedRuleResolver rulevalidation.AuthorizationRuleResolver) (*AuthorizationStorage, error) {
	policyStorage, err := policyetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	policyRegistry := policyregistry.NewRegistry(policyStorage)

	policyBindingStorage, err := policybindingetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	policyBindingRegistry := policybindingregistry.NewRegistry(policyBindingStorage)

	clusterPolicyStorage, err := clusterpolicyetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterPolicyStorage)

	clusterPolicyBindingStorage, err := clusterpolicybindingetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterPolicyBindingStorage)

	liveRuleResolver := NewLiveRuleResolver(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry)

	roleStorage := rolestorage.NewVirtualStorage(policyRegistry, liveRuleResolver, cachedRuleResolver)
	roleBindingStorage := rolebindingstorage.NewVirtualStorage(policyBindingRegistry, liveRuleResolver, cachedRuleResolver)
	clusterRoleStorage := clusterrolestorage.NewClusterRoleStorage(clusterPolicyRegistry, liveRuleResolver, cachedRuleResolver)
	clusterRoleBindingStorage := clusterrolebindingstorage.NewClusterRoleBindingStorage(clusterPolicyBindingRegistry, liveRuleResolver, cachedRuleResolver)

	return &AuthorizationStorage{
		Policy:               policyStorage,
		PolicyBinding:        policyBindingStorage,
		ClusterPolicy:        clusterPolicyStorage,
		ClusterPolicyBinding: clusterPolicyBindingStorage,
		Role:                 roleStorage,
		RoleBinding:          roleBindingStorage,
		ClusterRole:          clusterRoleStorage,
		ClusterRoleBinding:   clusterRoleBindingStorage,
	}, nil
}

type AuthorizationStorage struct {
	Policy               policyregistry.Storage
	PolicyBinding        policybindingregistry.Storage
	ClusterPolicy        clusterpolicyregistry.Storage
	ClusterPolicyBinding clusterpolicybindingregistry.Storage
	Role                 role.Storage
	RoleBinding          rolebinding.Storage
	ClusterRole          clusterrole.Storage
	ClusterRoleBinding   clusterrolebinding.Storage
}
