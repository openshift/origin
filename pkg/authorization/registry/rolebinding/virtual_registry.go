package rolebinding

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

// TODO sort out resourceVersions.  Perhaps a hash of the object contents?

type VirtualRegistry struct {
	bindingRegistry              policybindingregistry.Registry
	policyRegistry               policyregistry.Registry
	masterAuthorizationNamespace string
}

// NewVirtualRegistry creates a new REST for policies.
func NewVirtualRegistry(bindingRegistry policybindingregistry.Registry, policyRegistry policyregistry.Registry, masterAuthorizationNamespace string) Registry {
	return &VirtualRegistry{bindingRegistry, policyRegistry, masterAuthorizationNamespace}
}

// TODO either add selector for fields ot eliminate the option
func (m *VirtualRegistry) ListRoleBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.RoleBindingList, error) {
	policyBindingList, err := m.bindingRegistry.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	roleBindingList := &authorizationapi.RoleBindingList{}

	for _, policyBinding := range policyBindingList.Items {
		for _, roleBinding := range policyBinding.RoleBindings {
			if label.Matches(labels.Set(roleBinding.Labels)) {
				roleBindingList.Items = append(roleBindingList.Items, roleBinding)
			}
		}
	}

	return roleBindingList, nil
}

func (m *VirtualRegistry) GetRoleBinding(ctx kapi.Context, name string) (*authorizationapi.RoleBinding, error) {
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
	return &binding, nil
}

func (m *VirtualRegistry) DeleteRoleBinding(ctx kapi.Context, name string) error {
	owningPolicyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if err != nil && kapierrors.IsNotFound(err) {
		return kapierrors.NewNotFound("RoleBinding", name)
	}
	if err != nil {
		return err
	}

	if _, exists := owningPolicyBinding.RoleBindings[name]; !exists {
		return kapierrors.NewNotFound("RoleBinding", name)
	}

	delete(owningPolicyBinding.RoleBindings, name)
	owningPolicyBinding.LastModified = util.Now()

	return m.bindingRegistry.UpdatePolicyBinding(ctx, owningPolicyBinding)
}

func (m *VirtualRegistry) CreateRoleBinding(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding, allowEscalation bool) error {
	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return err
	}
	if !allowEscalation {
		if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
			return err
		}
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
	if err != nil {
		return err
	}

	_, exists := policyBinding.RoleBindings[roleBinding.Name]
	if exists {
		return kapierrors.NewAlreadyExists("RoleBinding", roleBinding.Name)
	}

	policyBinding.RoleBindings[roleBinding.Name] = *roleBinding
	policyBinding.LastModified = util.Now()

	if err := m.bindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return err
	}
	return nil
}

func (m *VirtualRegistry) UpdateRoleBinding(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding, allowEscalation bool) error {
	if err := m.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return err
	}
	if !allowEscalation {
		if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
			return err
		}
	}

	existingRoleBinding, err := m.GetRoleBinding(ctx, roleBinding.Name)
	if err != nil {
		return err
	}
	if existingRoleBinding == nil {
		return kapierrors.NewNotFound("RoleBinding", roleBinding.Name)
	}
	if existingRoleBinding.RoleRef.Namespace != roleBinding.RoleRef.Namespace {
		return fmt.Errorf("cannot change roleBinding.RoleRef.Namespace from %v to %v", existingRoleBinding.RoleRef.Namespace, roleBinding.RoleRef.Namespace)
	}

	policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
	if err != nil {
		return err
	}

	previousRoleBinding, exists := policyBinding.RoleBindings[roleBinding.Name]
	if !exists {
		return kapierrors.NewNotFound("RoleBinding", roleBinding.Name)
	}
	if previousRoleBinding.RoleRef != roleBinding.RoleRef {
		return errors.New("roleBinding.RoleRef may not be modified")
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
		return nil, err
	}

	role, exists := policy.Roles[roleRef.Name]
	if !exists {
		return nil, kapierrors.NewNotFound("Role", roleRef.Name)
	}

	return &role, nil
}

func (m *VirtualRegistry) confirmNoEscalation(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
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
func (m *VirtualRegistry) ensurePolicyBindingToMaster(ctx kapi.Context, policyNamespace string) (*authorizationapi.PolicyBinding, error) {
	policyBinding, err := m.bindingRegistry.GetPolicyBinding(ctx, policyNamespace)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}

		// if we have no policyBinding, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policyBinding = policybindingregistry.NewEmptyPolicyBinding(kapi.NamespaceValue(ctx), policyNamespace)
		if err := m.bindingRegistry.CreatePolicyBinding(ctx, policyBinding); err != nil {
			return nil, err
		}

		policyBinding, err = m.bindingRegistry.GetPolicyBinding(ctx, policyNamespace)
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
func (m *VirtualRegistry) getPolicyBindingForPolicy(ctx kapi.Context, policyNamespace string, allowAutoProvision bool) (*authorizationapi.PolicyBinding, error) {
	// we can autocreate a PolicyBinding object if the RoleBinding is for the master namespace OR if we've been explicity told to create the policying binding.
	// the latter happens during priming
	if (policyNamespace == m.masterAuthorizationNamespace) || allowAutoProvision {
		return m.ensurePolicyBindingToMaster(ctx, policyNamespace)
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
	policyBindingList, err := m.bindingRegistry.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
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
