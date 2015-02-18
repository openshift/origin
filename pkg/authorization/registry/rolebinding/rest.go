package rolebinding

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	klabels "github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/api/validation"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
)

// TODO add get and list
// TODO prevent privilege escalation

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	bindingRegistry              policybindingregistry.Registry
	policyRegistry               policyregistry.Registry
	userRegistry                 userregistry.Registry
	masterAuthorizationNamespace string
}

// NewREST creates a new REST for policies.
func NewREST(bindingRegistry policybindingregistry.Registry, policyRegistry policyregistry.Registry, userRegistry userregistry.Registry, masterAuthorizationNamespace string) apiserver.RESTStorage {
	return &REST{bindingRegistry, policyRegistry, userRegistry, masterAuthorizationNamespace}
}

// New creates a new RoleBinding object
func (r *REST) New() runtime.Object {
	return &authorizationapi.RoleBinding{}
}

// Delete asynchronously deletes the Policy specified by its id.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	owningPolicyBinding, err := r.LocatePolicyBinding(ctx, id)
	if err != nil {
		return nil, err
	}
	if owningPolicyBinding == nil {
		return nil, fmt.Errorf("roleBinding %v does not exist", id)
	}

	delete(owningPolicyBinding.RoleBindings, id)
	owningPolicyBinding.LastModified = util.Now()

	return &kapi.Status{Status: kapi.StatusSuccess}, r.bindingRegistry.UpdatePolicyBinding(ctx, owningPolicyBinding)
}

// Create registers a given new RoleBinding inside the Policy instance to r.bindingRegistry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	roleBinding, ok := obj.(*authorizationapi.RoleBinding)
	if !ok {
		return nil, fmt.Errorf("not a roleBinding: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &roleBinding.ObjectMeta) {
		return nil, kerrors.NewConflict("roleBinding", roleBinding.Namespace, fmt.Errorf("RoleBinding.Namespace does not match the provided context"))
	}

	kapi.FillObjectMetaSystemFields(ctx, &roleBinding.ObjectMeta)
	if errs := validation.ValidateRoleBinding(roleBinding); len(errs) > 0 {
		return nil, kerrors.NewInvalid("roleBinding", roleBinding.Name, errs)
	}

	if err := r.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return nil, err
	}
	if err := r.confirmRoleBindingNameUnique(ctx, roleBinding.Name); err != nil {
		return nil, err
	}

	policyBinding, err := r.GetPolicyBinding(ctx, roleBinding.RoleRef.Namespace)
	if err != nil {
		return nil, err
	}

	_, exists := policyBinding.RoleBindings[roleBinding.Name]
	if exists {
		return nil, fmt.Errorf("roleBinding %v already exists", roleBinding.Name)
	}

	policyBinding.RoleBindings[roleBinding.Name] = *roleBinding
	policyBinding.LastModified = util.Now()

	if err := r.bindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return nil, err
	}
	return roleBinding, nil
}

// Update replaces a given RoleBinding inside the Policy instance with an existing instance in r.bindingRegistry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	roleBinding, ok := obj.(*authorizationapi.RoleBinding)
	if !ok {
		return nil, false, fmt.Errorf("not a roleBinding: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &roleBinding.ObjectMeta) {
		return nil, false, kerrors.NewConflict("roleBinding", roleBinding.Namespace, fmt.Errorf("RoleBinding.Namespace does not match the provided context"))
	}

	if errs := validation.ValidateRoleBinding(roleBinding); len(errs) > 0 {
		return nil, false, kerrors.NewInvalid("roleBinding", roleBinding.Name, errs)
	}

	if err := r.validateReferentialIntegrity(ctx, roleBinding); err != nil {
		return nil, false, err
	}

	existingRoleBinding, err := r.GetRoleBinding(ctx, roleBinding.Name)
	if err != nil {
		return nil, false, err
	}
	if existingRoleBinding == nil {
		return nil, false, fmt.Errorf("roleBinding %v does not exist", roleBinding.Name)
	}
	if existingRoleBinding.RoleRef.Namespace != roleBinding.RoleRef.Namespace {
		return nil, false, fmt.Errorf("cannot change roleBinding.RoleRef.Namespace from %v to %v", existingRoleBinding.RoleRef.Namespace, roleBinding.RoleRef.Namespace)
	}

	policyBinding, err := r.GetPolicyBinding(ctx, roleBinding.RoleRef.Namespace)
	if err != nil {
		return nil, false, err
	}

	_, exists := policyBinding.RoleBindings[roleBinding.Name]
	if !exists {
		return nil, false, fmt.Errorf("roleBinding %v does not exist", roleBinding.Name)
	}

	policyBinding.RoleBindings[roleBinding.Name] = *roleBinding
	policyBinding.LastModified = util.Now()

	if err := r.bindingRegistry.UpdatePolicyBinding(ctx, policyBinding); err != nil {
		return nil, false, err
	}
	return roleBinding, false, nil
}

func (r *REST) validateReferentialIntegrity(ctx kapi.Context, roleBinding *authorizationapi.RoleBinding) error {
	if err := r.confirmRoleExists(roleBinding.RoleRef); err != nil {
		return err
	}

	return nil
}

func (r *REST) confirmRoleBindingNameUnique(ctx kapi.Context, bindingName string) error {
	policyBinding, err := r.LocatePolicyBinding(ctx, bindingName)
	if err != nil {
		return err
	}

	if policyBinding != nil {
		return fmt.Errorf("%v already exists", bindingName)
	}

	return nil
}

func (r *REST) confirmRoleExists(roleRef kapi.ObjectReference) error {
	ctx := kapi.WithNamespace(kapi.NewContext(), roleRef.Namespace)

	policy, err := r.policyRegistry.GetPolicy(ctx, authorizationapi.PolicyName)
	if err != nil {
		return fmt.Errorf("policy %v not found: %v", roleRef.Namespace, err)
	}

	if _, exists := policy.Roles[roleRef.Name]; !exists {
		return fmt.Errorf("role %v not found", roleRef.Name)
	}

	return nil
}

func (r *REST) confirmUsersExist(userNames []string) error {
	for _, userName := range userNames {
		if _, err := r.userRegistry.GetUser(userName); err != nil {
			return fmt.Errorf("user %v not found: %v", userName, err)
		}
	}

	return nil
}

// EnsurePolicyBindingToMaster returns a PolicyBinding object that has a PolicyRef pointing to the Policy in the passed namespace.
func (r *REST) EnsurePolicyBindingToMaster(ctx kapi.Context) (*authorizationapi.PolicyBinding, error) {
	policyBinding, err := r.bindingRegistry.GetPolicyBinding(ctx, r.masterAuthorizationNamespace)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, err
		}

		// if we have no policyBinding, go ahead and make one.  creating one here collapses code paths below.  We only take this hit once
		policyBinding = policybindingregistry.NewEmptyPolicyBinding(kapi.NamespaceValue(ctx), r.masterAuthorizationNamespace)
		if err := r.bindingRegistry.CreatePolicyBinding(ctx, policyBinding); err != nil {
			return nil, err
		}

		policyBinding, err = r.bindingRegistry.GetPolicyBinding(ctx, r.masterAuthorizationNamespace)
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
func (r *REST) GetPolicyBinding(ctx kapi.Context, policyNamespace string) (*authorizationapi.PolicyBinding, error) {
	// we can autocreate a PolicyBinding object if the RoleBinding is for the master namespace
	if policyNamespace == r.masterAuthorizationNamespace {
		return r.EnsurePolicyBindingToMaster(ctx)
	}

	policyBinding, err := r.bindingRegistry.GetPolicyBinding(ctx, policyNamespace)
	if err != nil {
		return nil, err
	}

	if policyBinding.RoleBindings == nil {
		policyBinding.RoleBindings = make(map[string]authorizationapi.RoleBinding)
	}

	return policyBinding, nil
}

func (r *REST) LocatePolicyBinding(ctx kapi.Context, roleBindingName string) (*authorizationapi.PolicyBinding, error) {
	policyBindingList, err := r.bindingRegistry.ListPolicyBindings(ctx, klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	for _, policyBinding := range policyBindingList.Items {
		_, exists := policyBinding.RoleBindings[roleBindingName]
		if exists {
			return &policyBinding, nil
		}
	}

	return nil, nil
}

func (r *REST) GetRoleBinding(ctx kapi.Context, roleBindingName string) (*authorizationapi.RoleBinding, error) {
	policyBindingList, err := r.bindingRegistry.ListPolicyBindings(ctx, klabels.Everything(), klabels.Everything())
	if err != nil {
		return nil, err
	}

	for _, policyBinding := range policyBindingList.Items {
		roleBinding, exists := policyBinding.RoleBindings[roleBindingName]
		if exists {
			return &roleBinding, nil
		}
	}

	return nil, nil
}
