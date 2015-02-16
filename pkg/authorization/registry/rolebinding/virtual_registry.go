package rolebinding

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type VirtualRegistry struct {
	bindingRegistry              policybindingregistry.Registry
	policyRegistry               policyregistry.Registry
	masterAuthorizationNamespace string
}

// NewVirtualRegistry creates a new REST for policies.
func NewVirtualRegistry(bindingRegistry policybindingregistry.Registry, policyRegistry policyregistry.Registry, masterAuthorizationNamespace string) Registry {
	return &VirtualRegistry{bindingRegistry, policyRegistry, masterAuthorizationNamespace}
}

func (m *VirtualRegistry) ListRoleBindings(ctx kapi.Context, labels, fields klabels.Selector) (*authorizationapi.RoleBindingList, error) {
	policyBindingList, err := m.bindingRegistry.ListPolicyBindings(ctx, klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	roleBindingList := &authorizationapi.RoleBindingList{}

	for _, policyBinding := range policyBindingList.Items {
		for _, roleBinding := range policyBinding.RoleBindings {
			roleBindingList.Items = append(roleBindingList.Items, roleBinding)
		}
	}

	return roleBindingList, nil
}

func (m *VirtualRegistry) GetRoleBinding(ctx kapi.Context, name string) (*authorizationapi.RoleBinding, error) {
	policyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if err != nil {
		return nil, err
	}
	if policyBinding == nil {
		return nil, fmt.Errorf("RoleBinding %v not found", name)
	}

	binding := policyBinding.RoleBindings[name]
	return &binding, nil
}

func (m *VirtualRegistry) DeleteRoleBinding(ctx kapi.Context, name string) error {
	owningPolicyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if err != nil {
		return err
	}
	if owningPolicyBinding == nil {
		return fmt.Errorf("roleBinding %v does not exist", name)
	}

	delete(owningPolicyBinding.RoleBindings, name)
	owningPolicyBinding.LastModified = util.Now()

	return m.bindingRegistry.UpdatePolicyBinding(ctx, owningPolicyBinding)
}

func (m *VirtualRegistry) CreateRoleBinding(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return err
	}
	if err := m.confirmNoEscalaton(ctx, roleBinding); err != nil {
		return err
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace)
	if err != nil {
		return err
	}

	_, exists := policyBinding.RoleBindings[roleBinding.Name]
	if exists {
		return fmt.Errorf("roleBinding %v already exists", roleBinding.Name)
	}

	policyBinding.RoleBindings[roleBinding.Name] = *roleBinding
	policyBinding.LastModified = util.Now()

	if err := m.bindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return err
	}
	return nil
}

func (m *VirtualRegistry) UpdateRoleBinding(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return err
	}
	if err := m.confirmNoEscalaton(ctx, roleBinding); err != nil {
		return err
	}

	existingRoleBinding, err := m.GetRoleBinding(ctx, roleBinding.Name)
	if err != nil {
		return err
	}
	if existingRoleBinding == nil {
		return fmt.Errorf("roleBinding %v does not exist", roleBinding.Name)
	}
	if existingRoleBinding.RoleRef.Namespace != roleBinding.RoleRef.Namespace {
		return fmt.Errorf("cannot change roleBinding.RoleRef.Namespace from %v to %v", existingRoleBinding.RoleRef.Namespace, roleBinding.RoleRef.Namespace)
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace)
	if err != nil {
		return err
	}

	_, exists := policyBinding.RoleBindings[roleBinding.Name]
	if !exists {
		return fmt.Errorf("roleBinding %v does not exist", roleBinding.Name)
	}

	policyBinding.RoleBindings[roleBinding.Name] = *roleBinding
	policyBinding.LastModified = util.Now()

	if err := m.bindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return err
	}
	return nil
}

func (m *VirtualRegistry) validateReferentialIntegrity(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	if _, err := m.getReferencedRole(roleBinding.RoleRef); err != nil {
		return err
	}

	return nil
}

func (m *VirtualRegistry) getReferencedRole(roleRef kapi.ObjectReference) (*authorizationapi.Role, error) {
	ctx := kapi.WithNamespace(kapi.NewContext(), roleRef.Namespace)

	policy, err := m.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil {
		return nil, fmt.Errorf("policy %v not found: %v", roleRef.Namespace, err)
	}

	role, exists := policy.Roles[roleRef.Name]
	if !exists {
		return nil, fmt.Errorf("role %v not found", roleRef.Name)
	}

	return &role, nil
}

func (m *VirtualRegistry) confirmNoEscalaton(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	modifyingRole, err := m.getReferencedRole(roleBinding.RoleRef)
	if err != nil {
		return err
	}

	ruleResolver := rulevalidation.NewDefaultRuleResolver(m.policyRegistry, m.bindingRegistry)
	ownerLocalRules, err := ruleResolver.GetEffectivePolicyRules(ctx)
	if err != nil {
		return err
	}
	masterContext := kapi.WithNamespace(ctx, m.masterAuthorizationNamespace)
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
func (m *VirtualRegistry) ensurePolicyBindingToMaster(ctx kapi.Context) (*authorizationapi.PolicyBinding, error) {
	policyBinding, err := m.bindingRegistry.GetPolicyBinding(ctx, m.masterAuthorizationNamespace)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, err
		}

		// if we have no policyBinding, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policyBinding = policybindingregistry.NewEmptyPolicyBinding(kapi.NamespaceValue(ctx), m.masterAuthorizationNamespace)
		if err := m.bindingRegistry.CreatePolicyBinding(ctx, policyBinding); err != nil {
			return nil, err
		}

		policyBinding, err = m.bindingRegistry.GetPolicyBinding(ctx, m.masterAuthorizationNamespace)
		if err != nil {
			return nil, err
		}
	}

	if policyBinding.RoleBindings == nil {
		policyBinding.RoleBindings = make(map[string]authorizationapi.RoleBinding)
	}

	return policyBinding, nil
}

// Returns a PolicyBinding that points to the specified policyNamespace.  It will autocreate ONLY if policyNamespace equals the master namespace
func (m *VirtualRegistry) getPolicyBindingForPolicy(ctx kapi.Context, policyNamespace string) (*authorizationapi.PolicyBinding, error) {
	// we can autocreate a PolicyBinding object if the RoleBinding is for the master namespace
	if policyNamespace == m.masterAuthorizationNamespace {
		return m.ensurePolicyBindingToMaster(ctx)
	}

	policyBinding, err := m.bindingRegistry.GetPolicyBinding(ctx, policyNamespace)
	if err != nil {
		return nil, err
	}

	if policyBinding.RoleBindings == nil {
		policyBinding.RoleBindings = make(map[string]authorizationapi.RoleBinding)
	}

	return policyBinding, nil
}

func (m *VirtualRegistry) getPolicyBindingOwningRoleBinding(ctx kapi.Context, bindingName string) (*authorizationapi.PolicyBinding, error) {
	policyBindingList, err := m.bindingRegistry.ListPolicyBindings(ctx, klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	for _, policyBinding := range policyBindingList.Items {
		_, exists := policyBinding.RoleBindings[bindingName]
		if exists {
			return &policyBinding, nil
		}
	}

	return nil, nil
}
