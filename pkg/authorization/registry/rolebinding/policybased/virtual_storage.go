package policybased

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationinterfaces "github.com/openshift/origin/pkg/authorization/interfaces"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type VirtualStorage struct {
	BindingRegistry policybindingregistry.Registry

	RuleResolver       rulevalidation.AuthorizationRuleResolver
	CachedRuleResolver rulevalidation.AuthorizationRuleResolver

	CreateStrategy rest.RESTCreateStrategy
	UpdateStrategy rest.RESTUpdateStrategy
	Resource       unversioned.GroupResource
}

// NewVirtualStorage creates a new REST for policies.
func NewVirtualStorage(bindingRegistry policybindingregistry.Registry, ruleResolver, cachedRuleResolver rulevalidation.AuthorizationRuleResolver, resource unversioned.GroupResource) rolebindingregistry.Storage {
	return &VirtualStorage{
		BindingRegistry: bindingRegistry,

		RuleResolver:       ruleResolver,
		CachedRuleResolver: cachedRuleResolver,

		CreateStrategy: rolebindingregistry.LocalStrategy,
		UpdateStrategy: rolebindingregistry.LocalStrategy,
		Resource:       resource,
	}
}

func (m *VirtualStorage) New() runtime.Object {
	return &authorizationapi.RoleBinding{}
}
func (m *VirtualStorage) NewList() runtime.Object {
	return &authorizationapi.RoleBindingList{}
}

func (m *VirtualStorage) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	policyBindingList, err := m.BindingRegistry.ListPolicyBindings(ctx, options)
	if err != nil {
		return nil, err
	}

	labelSelector, fieldSelector := oapi.ListOptionsToSelectors(options)

	roleBindingList := &authorizationapi.RoleBindingList{}
	for _, policyBinding := range policyBindingList.Items {
		for _, roleBinding := range policyBinding.RoleBindings {
			if labelSelector.Matches(labels.Set(roleBinding.Labels)) &&
				fieldSelector.Matches(authorizationapi.RoleBindingToSelectableFields(roleBinding)) {
				roleBindingList.Items = append(roleBindingList.Items, *roleBinding)
			}
		}
	}

	return roleBindingList, nil
}

func (m *VirtualStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	policyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
	if kapierrors.IsNotFound(err) {
		return nil, kapierrors.NewNotFound(m.Resource, name)
	}
	if err != nil {
		return nil, err
	}

	binding, exists := policyBinding.RoleBindings[name]
	if !exists {
		return nil, kapierrors.NewNotFound(m.Resource, name)
	}
	return binding, nil
}

func (m *VirtualStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	if err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
		owningPolicyBinding, err := m.getPolicyBindingOwningRoleBinding(ctx, name)
		if kapierrors.IsNotFound(err) {
			return kapierrors.NewNotFound(m.Resource, name)
		}
		if err != nil {
			return err
		}

		if _, exists := owningPolicyBinding.RoleBindings[name]; !exists {
			return kapierrors.NewNotFound(m.Resource, name)
		}

		delete(owningPolicyBinding.RoleBindings, name)
		owningPolicyBinding.LastModified = unversioned.Now()

		return m.BindingRegistry.UpdatePolicyBinding(ctx, owningPolicyBinding)
	}); err != nil {
		return nil, err
	}

	return &unversioned.Status{Status: unversioned.StatusSuccess}, nil
}

func (m *VirtualStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return m.createRoleBinding(ctx, obj, false)
}

func (m *VirtualStorage) CreateRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, error) {
	return m.createRoleBinding(ctx, obj, true)
}

func (m *VirtualStorage) createRoleBinding(ctx kapi.Context, obj runtime.Object, allowEscalation bool) (*authorizationapi.RoleBinding, error) {
	// Copy object before passing to BeforeCreate, since it mutates
	objCopy, err := kapi.Scheme.DeepCopy(obj)
	if err != nil {
		return nil, err
	}
	obj = objCopy.(runtime.Object)

	if err := rest.BeforeCreate(m.CreateStrategy, ctx, obj); err != nil {
		return nil, err
	}

	roleBinding := obj.(*authorizationapi.RoleBinding)

	if !allowEscalation {
		if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
			return nil, err
		}
	}

	// Retry if we hit a conflict on the underlying PolicyBinding object
	if err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
		policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
		if err != nil {
			return err
		}

		_, exists := policyBinding.RoleBindings[roleBinding.Name]
		if exists {
			return kapierrors.NewAlreadyExists(m.Resource, roleBinding.Name)
		}

		roleBinding.ResourceVersion = policyBinding.ResourceVersion
		policyBinding.RoleBindings[roleBinding.Name] = roleBinding
		policyBinding.LastModified = unversioned.Now()

		return m.BindingRegistry.UpdatePolicyBinding(ctx, policyBinding)
	}); err != nil {
		return nil, err
	}

	return roleBinding, nil
}

func (m *VirtualStorage) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	return m.updateRoleBinding(ctx, name, objInfo, false)
}
func (m *VirtualStorage) UpdateRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.RoleBinding) (*authorizationapi.RoleBinding, bool, error) {
	return m.updateRoleBinding(ctx, obj.Name, rest.DefaultUpdatedObjectInfo(obj, kapi.Scheme), true)
}

func (m *VirtualStorage) updateRoleBinding(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo, allowEscalation bool) (*authorizationapi.RoleBinding, bool, error) {
	var updatedRoleBinding *authorizationapi.RoleBinding
	var roleBindingConflicted = false

	if err := kclient.RetryOnConflict(kclient.DefaultRetry, func() error {
		// Do an initial fetch
		old, err := m.Get(ctx, name)
		if err != nil {
			return err
		}
		oldRoleBinding, exists := old.(*authorizationapi.RoleBinding)
		if !exists {
			return kapierrors.NewBadRequest(fmt.Sprintf("old obj is not a role binding: %#v", old))
		}

		// get the updated object, so we know what namespace we're binding against
		obj, err := objInfo.UpdatedObject(ctx, old)
		if err != nil {
			return err
		}
		roleBinding, ok := obj.(*authorizationapi.RoleBinding)
		if !ok {
			return kapierrors.NewBadRequest(fmt.Sprintf("obj is not a role binding: %#v", obj))
		}

		// now that we know which roleRef we want to go to, fetch the policyBinding we'll actually be updating, and re-get the oldRoleBinding
		policyBinding, err := m.getPolicyBindingForPolicy(ctx, roleBinding.RoleRef.Namespace, allowEscalation)
		if err != nil {
			return err
		}
		oldRoleBinding, exists = policyBinding.RoleBindings[roleBinding.Name]
		if !exists {
			return kapierrors.NewNotFound(m.Resource, roleBinding.Name)
		}

		if len(roleBinding.ResourceVersion) == 0 && m.UpdateStrategy.AllowUnconditionalUpdate() {
			roleBinding.ResourceVersion = oldRoleBinding.ResourceVersion
		}

		if err := rest.BeforeUpdate(m.UpdateStrategy, ctx, obj, oldRoleBinding); err != nil {
			return err
		}

		if !allowEscalation {
			if err := m.confirmNoEscalation(ctx, roleBinding); err != nil {
				return err
			}
		}

		// conflict detection
		if roleBinding.ResourceVersion != oldRoleBinding.ResourceVersion {
			// mark as a conflict err, but return an untyped error to escape the retry
			roleBindingConflicted = true
			return errors.New(registry.OptimisticLockErrorMsg)
		}
		// non-mutating change
		if kapi.Semantic.DeepEqual(oldRoleBinding, roleBinding) {
			updatedRoleBinding = roleBinding
			return nil
		}

		roleBinding.ResourceVersion = policyBinding.ResourceVersion
		policyBinding.RoleBindings[roleBinding.Name] = roleBinding
		policyBinding.LastModified = unversioned.Now()

		if err := m.BindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
			return err
		}
		updatedRoleBinding = roleBinding
		return nil
	}); err != nil {
		if roleBindingConflicted {
			// construct the typed conflict error
			return nil, false, kapierrors.NewConflict(m.Resource, name, err)
		}
		return nil, false, err
	}
	return updatedRoleBinding, false, nil
}

// roleForEscalationCheck tries to use the CachedRuleResolver if available to avoid expensive checks
func (m *VirtualStorage) roleForEscalationCheck(binding authorizationinterfaces.RoleBinding) (authorizationinterfaces.Role, error) {
	if m.CachedRuleResolver != nil {
		if role, err := m.CachedRuleResolver.GetRole(binding); err == nil {
			return role, nil
		}
	}
	return m.RuleResolver.GetRole(binding)
}

func (m *VirtualStorage) confirmNoEscalation(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	modifyingRole, err := m.roleForEscalationCheck(authorizationinterfaces.NewLocalRoleBindingAdapter(roleBinding))
	if err != nil {
		return err
	}

	return rulevalidation.ConfirmNoEscalation(ctx, m.Resource, roleBinding.Name, m.RuleResolver, m.CachedRuleResolver, modifyingRole)
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
			// Tolerate the policybinding having been created in the meantime
			if !kapierrors.IsAlreadyExists(err) {
				return nil, err
			}
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
	// we can autocreate a PolicyBinding object if the RoleBinding is for the master namespace OR if we've been explicitly told to create the policying binding.
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
	policyBindingList, err := m.BindingRegistry.ListPolicyBindings(ctx, &kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, policyBinding := range policyBindingList.Items {
		_, exists := policyBinding.RoleBindings[bindingName]
		if exists {
			return &policyBinding, nil
		}
	}

	return nil, kapierrors.NewNotFound(m.Resource, bindingName)
}
