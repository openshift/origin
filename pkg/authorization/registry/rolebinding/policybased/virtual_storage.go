package policybased

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type VirtualStorage struct {
	PolicyRegistry               policyregistry.Registry
	BindingRegistry              policybindingregistry.Registry
	ClusterPolicyRegistry        clusterpolicyregistry.Registry
	ClusterPolicyBindingRegistry clusterpolicybindingregistry.Registry

	CreateStrategy rest.RESTCreateStrategy
	UpdateStrategy rest.RESTUpdateStrategy
}

// NewVirtualStorage creates a new REST for policies.
func NewVirtualStorage(policyRegistry policyregistry.Registry, bindingRegistry policybindingregistry.Registry, clusterPolicyRegistry clusterpolicyregistry.Registry, clusterPolicyBindingRegistry clusterpolicybindingregistry.Registry) rolebindingregistry.Storage {
	return &VirtualStorage{
		PolicyRegistry:               policyRegistry,
		BindingRegistry:              bindingRegistry,
		ClusterPolicyRegistry:        clusterPolicyRegistry,
		ClusterPolicyBindingRegistry: clusterPolicyBindingRegistry,

		CreateStrategy: rolebindingregistry.LocalStrategy,
		UpdateStrategy: rolebindingregistry.LocalStrategy,
	}
}

func (m *VirtualStorage) New() runtime.Object {
	return &authorizationapi.RoleBinding{}
}
func (m *VirtualStorage) NewList() runtime.Object {
	return &authorizationapi.RoleBindingList{}
}

// TODO either add selector for fields ot eliminate the option
func (m *VirtualStorage) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	policyBindingList, err := m.BindingRegistry.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	roleBindingList := &authorizationapi.RoleBindingList{}

	for _, policyBinding := range policyBindingList.Items {
		for _, roleBinding := range policyBinding.RoleBindings {
			if label.Matches(labels.Set(roleBinding.Labels)) {
				roleBindingList.Items = append(roleBindingList.Items, *roleBinding)
			}
		}
	}

	return roleBindingList, nil
}

func (m *VirtualStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	policyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if err != nil && kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound("RoleBinding", name)
	}
	if err != nil {
		return nil, err
	}

	binding, exists := policyBinding.RoleBindings[name]
	if !exists {
		return nil, kapierrors.NewNotFound("RoleBinding", name)
	}
	return binding, nil
}

func (m *VirtualStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	owningPolicyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if err != nil && kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound("RoleBinding", name)
	}
	if err != nil {
		return nil, err
	}

	if _, exists := owningPolicyBinding.RoleBindings[name]; !exists {
		return nil, kapierrors.NewNotFound("RoleBinding", name)
	}

	delete(owningPolicyBinding.RoleBindings, name)
	owningPolicyBinding.LastModified = util.Now()

	if err := m.BindingRegistry.UpdatePolicyBinding(ctx, owningPolicyBinding); err != nil {
		return nil, err
	}

	return &kapi.Status{Status: kapi.StatusSuccess}, nil
}

func (m *VirtualStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return m.createRoleBinding(ctx, obj, false)
}

func (m *VirtualStorage) CreateRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	return m.createRoleBinding(ctx, obj, true)
}

func (m *VirtualStorage) createRoleBinding(ctx kapi.Context, obj runtime.Object, allowEscalation bool) (*authorizationapi.RoleBinding, error) {
	if err := rest.BeforeCreate(m.CreateStrategy, ctx, obj); err != nil {
		return nil, err
	}

	roleBinding := obj.(*authorizationapi.RoleBinding)

	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return nil, err
	}
	if !allowEscalation {
		if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
			return nil, err
		}
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
	if err != nil {
		return nil, err
	}

	_, exists := policyBinding.RoleBindings[roleBinding.Name]
	if exists {
		return nil, kapierrors.NewAlreadyExists("RoleBinding", roleBinding.Name)
	}

	roleBinding.ResourceVersion = policyBinding.ResourceVersion
	policyBinding.RoleBindings[roleBinding.Name] = roleBinding
	policyBinding.LastModified = util.Now()

	if err := m.BindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return nil, err
	}

	return roleBinding, nil
}

func (m *VirtualStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	return m.updateRoleBinding(ctx, obj, false)
}
func (m *VirtualStorage) UpdateRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, bool, error) {
	return m.updateRoleBinding(ctx, obj, true)
}

func (m *VirtualStorage) updateRoleBinding(ctx kapi.Context, obj runtime.Object, allowEscalation bool) (*authorizationapi.RoleBinding, bool, error) {
	roleBinding, ok := obj.(*authorizationapi.RoleBinding)
	if !ok {
		return nil, false, kapierrors.NewBadRequest(fmt.Sprintf("obj is not a role: %#v", obj))
	}

	old, err := m.Get(ctx, roleBinding.Name)
	if err != nil {
		return nil, false, err
	}

	if err := rest.BeforeUpdate(m.UpdateStrategy, ctx, obj, old); err != nil {
		return nil, false, err
	}

	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return nil, false, err
	}
	if !allowEscalation {
		if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
			return nil, false, err
		}
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
	if err != nil {
		return nil, false, err
	}

	previousRoleBinding, exists := policyBinding.RoleBindings[roleBinding.Name]
	if !exists {
		return nil, false, kapierrors.NewNotFound("RoleBinding", roleBinding.Name)
	}
	if previousRoleBinding.RoleRef != roleBinding.RoleRef {
		return nil, false, errors.New("roleBinding.RoleRef may not be modified")
	}

	roleBinding.ResourceVersion = policyBinding.ResourceVersion
	policyBinding.RoleBindings[roleBinding.Name] = roleBinding
	policyBinding.LastModified = util.Now()

	if err := m.BindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return nil, false, err
	}
	return roleBinding, false, nil
}

func (m *VirtualStorage) validateReferentialIntegrity(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	if _, err := m.getReferencedRole(roleBinding.RoleRef); err != nil {
		return err
	}

	return nil
}

func (m *VirtualStorage) getReferencedRole(roleRef kapi.ObjectReference) (*authorizationapi.Role, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), roleRef.Namespace)

	var policy *authorizationapi.Policy
	var err error
	switch {
	case len(roleRef.Namespace) == 0:
		var clusterPolicy *authorizationapi.ClusterPolicy
		clusterPolicy, err = m.ClusterPolicyRegistry.GetClusterPolicy(ctx, authorizationapi.PolicyName)
		policy = authorizationapi.ToPolicy(clusterPolicy)
	default:
		policy, err = m.PolicyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	}

	if err != nil {
		return nil, err
	}

	role, exists := policy.Roles[roleRef.Name]
	if !exists {
		return nil, kapierrors.NewNotFound("Role", roleRef.Name)
	}

	return role, nil
}

func (m *VirtualStorage) confirmNoEscalation(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	modifyingRole, err := m.getReferencedRole(roleBinding.RoleRef)
	if err != nil {
		return err
	}

	ruleResolver := rulevalidation.NewDefaultRuleResolver(
		m.PolicyRegistry,
		m.BindingRegistry,
		clusterpolicyregistry.NewSimulatedRegistry(m.ClusterPolicyRegistry),
		clusterpolicybindingregistry.NewSimulatedRegistry(m.ClusterPolicyBindingRegistry),
	)
	ownerLocalRules, err := ruleResolver.GetEffectivePolicyRules(ctx)
	if err != nil {
		return err
	}
	masterContext := kapi.WithNamespace(ctx, "")
	ownerGlobalRules, err := ruleResolver.GetEffectivePolicyRules(masterContext)
	if err != nil {
		return err
	}

	ownerRules := make([]authorizationapi.PolicyRule, 0, len(ownerGlobalRules)+len(ownerLocalRules))
	ownerRules = append(ownerRules, ownerLocalRules...)
	ownerRules = append(ownerRules, ownerGlobalRules...)

	ownerRightsCover, missingRights := rulevalidation.Covers(ownerRules, modifyingRole.Rules)
	if !ownerRightsCover {
		user, _ := kapi.UserFrom(ctx)
		return fmt.Errorf("attempt to grant extra privileges: %v\nuser=%v\nownerrules%v\n", missingRights, user, ownerRules)
	}

	return nil
}

// ensurePolicyBindingToMaster returns a PolicyBinding object that has a PolicyRef pointing to the Policy in the passed namespace.
func (m *VirtualStorage) ensurePolicyBindingToMaster(ctx kapi.Context, policyNamespace, policyBindingName string) (*authorizationapi.PolicyBinding, error) {
	policyBinding, err := m.BindingRegistry.GetPolicyBinding(ctx, policyBindingName)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}

		// if we have no policyBinding, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policyBinding = policybindingregistry.NewEmptyPolicyBinding(kapi.NamespaceValue(ctx), policyNamespace, policyBindingName)
		if err := m.BindingRegistry.CreatePolicyBinding(ctx, policyBinding); err != nil {
			return nil, err
		}

		policyBinding, err = m.BindingRegistry.GetPolicyBinding(ctx, policyBindingName)
		if err != nil {
			return nil, err
		}
	}

	if policyBinding.RoleBindings == nil {
		policyBinding.RoleBindings = make(map[string]*authorizationapi.RoleBinding)
	}

	return policyBinding, nil
}

// getPolicyBindingForPolicy returns a PolicyBinding that points to the specified policyNamespace.  It will autocreate ONLY if policyNamespace equals the master namespace
func (m *VirtualStorage) getPolicyBindingForPolicy(ctx kapi.Context, policyNamespace string, allowAutoProvision bool) (*authorizationapi.PolicyBinding, error) {
	// we can autocreate a PolicyBinding object if the RoleBinding is for the master namespace OR if we've been explicity told to create the policying binding.
	// the latter happens during priming
	if (policyNamespace == "") || allowAutoProvision {
		return m.ensurePolicyBindingToMaster(ctx, policyNamespace, authorizationapi.GetPolicyBindingName(policyNamespace))
	}

	policyBinding, err := m.BindingRegistry.GetPolicyBinding(ctx, authorizationapi.GetPolicyBindingName(policyNamespace))
	if err != nil {
		return nil, err
	}

	if policyBinding.RoleBindings == nil {
		policyBinding.RoleBindings = make(map[string]*authorizationapi.RoleBinding)
	}

	return policyBinding, nil
}

func (m *VirtualStorage) getPolicyBindingOwningRoleBinding(ctx kapi.Context, bindingName string) (*authorizationapi.PolicyBinding, error) {
	policyBindingList, err := m.BindingRegistry.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	for _, policyBinding := range policyBindingList.Items {
		_, exists := policyBinding.RoleBindings[bindingName]
		if exists {
			return &policyBinding, nil
		}
	}

	return nil, kapierrors.NewNotFound("RoleBinding", bindingName)
}
