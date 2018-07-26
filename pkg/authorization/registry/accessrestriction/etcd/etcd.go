package etcd

import (
	"context"
	"errors"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	rbacregistry "k8s.io/kubernetes/pkg/registry/rbac"
	rbacregistryvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/accessrestriction"
	"github.com/openshift/origin/pkg/util/restoptions"
)

// readStorage embeds all standard non-mutating storage interfaces
type readStorage interface {
	rest.Storage
	rest.Getter
	rest.Lister
	rest.Watcher
	rest.Exporter
	rest.TableConvertor
	rest.Scoper
}

type REST struct {
	// only embed read interfaces to make sure writes always check for escalation
	readStorage
	storage      rest.StandardStorage
	ruleResolver rbacregistryvalidation.AuthorizationRuleResolver
}

var (
	_ rest.StandardStorage = &REST{}

	groupResource = authorizationapi.Resource("accessrestrictions")

	fullAuthority = []rbacv1.PolicyRule{
		rbacv1helpers.NewRule("*").Groups("*").Resources("*").RuleOrDie(),
		rbacv1helpers.NewRule("*").URLs("*").RuleOrDie(),
	}

	errNotClusterAdmin = errors.New("must have cluster-admin privileges to write access restrictions")
)

// NewREST returns a RESTStorage object that will work against nodes.
func NewREST(optsGetter restoptions.Getter, ruleResolver rbacregistryvalidation.AuthorizationRuleResolver) (*REST, error) {
	store := &registry.Store{
		NewFunc:                  func() runtime.Object { return &authorizationapi.AccessRestriction{} },
		NewListFunc:              func() runtime.Object { return &authorizationapi.AccessRestrictionList{} },
		DefaultQualifiedResource: groupResource,

		CreateStrategy: accessrestriction.Strategy,
		UpdateStrategy: accessrestriction.Strategy,
		DeleteStrategy: accessrestriction.Strategy,
	}

	options := &generic.StoreOptions{RESTOptions: optsGetter}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &REST{readStorage: store, storage: store, ruleResolver: ruleResolver}, nil
}

func (s *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, includeUninitialized bool) (runtime.Object, error) {
	if err := s.confirmFullAuthority(ctx, obj.(*authorizationapi.AccessRestriction).Name); err != nil {
		return nil, err
	}

	return s.storage.Create(ctx, obj, createValidation, includeUninitialized)
}

func (s *REST) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	if err := s.confirmFullAuthority(ctx, name); err != nil {
		return nil, false, err
	}

	return s.storage.Update(ctx, name, objInfo, createValidation, updateValidation)
}

func (s *REST) Delete(ctx context.Context, name string, options *v1.DeleteOptions) (runtime.Object, bool, error) {
	if err := s.confirmFullAuthority(ctx, name); err != nil {
		return nil, false, err
	}

	return s.storage.Delete(ctx, name, options)
}

func (s *REST) DeleteCollection(ctx context.Context, options *v1.DeleteOptions, listOptions *internalversion.ListOptions) (runtime.Object, error) {
	if err := s.confirmFullAuthority(ctx, ""); err != nil {
		return nil, err
	}

	return s.storage.DeleteCollection(ctx, options, listOptions)
}

func (s *REST) confirmFullAuthority(ctx context.Context, name string) error {
	if rbacregistry.EscalationAllowed(ctx) {
		return nil
	}

	if err := rbacregistryvalidation.ConfirmNoEscalation(ctx, s.ruleResolver, fullAuthority); err != nil {
		return apierrors.NewForbidden(groupResource, name, errNotClusterAdmin)
	}

	return nil
}
