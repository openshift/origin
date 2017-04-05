package proxy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/client/restclient"
	nsregistry "k8s.io/kubernetes/pkg/registry/core/namespace"
	"k8s.io/kubernetes/pkg/runtime"
	kstorage "k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/watch"

	oapi "github.com/openshift/origin/pkg/api"
	authclient "github.com/openshift/origin/pkg/auth/client"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/project/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/project/api/validation"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project"
	projectutil "github.com/openshift/origin/pkg/project/util"
)

type REST struct {
	// client can modify Kubernetes namespaces
	client kcoreclient.NamespaceInterface
	// config to build a client that can impersonate users
	config restclient.Config
	// lister can enumerate project lists that enforce policy
	lister projectauth.Lister
	// Allows extended behavior during creation, required
	createStrategy rest.RESTCreateStrategy
	// Allows extended behavior during updates, required
	updateStrategy rest.RESTUpdateStrategy

	authCache    *projectauth.AuthorizationCache
	projectCache *projectcache.ProjectCache
}

// NewREST returns a RESTStorage object that will work against Project resources
func NewREST(client kcoreclient.NamespaceInterface, privilegedConfig restclient.Config, lister projectauth.Lister, authCache *projectauth.AuthorizationCache, projectCache *projectcache.ProjectCache) *REST {
	return &REST{
		client:         client,
		config:         privilegedConfig,
		lister:         lister,
		createStrategy: projectregistry.Strategy,
		updateStrategy: projectregistry.Strategy,

		authCache:    authCache,
		projectCache: projectCache,
	}
}

// New returns a new Project
func (s *REST) New() runtime.Object {
	return &api.Project{}
}

// NewList returns a new ProjectList
func (*REST) NewList() runtime.Object {
	return &api.ProjectList{}
}

var _ = rest.Lister(&REST{})

// List retrieves a list of Projects that match label.
func (s *REST) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden(projectapi.Resource("project"), "", fmt.Errorf("unable to list projects without a user on the context"))
	}
	namespaceList, err := s.lister.List(user)
	if err != nil {
		return nil, err
	}
	m := nsregistry.MatchNamespace(oapi.ListOptionsToSelectors(options))
	list, err := filterList(namespaceList, m, nil)
	if err != nil {
		return nil, err
	}
	return projectutil.ConvertNamespaceList(list.(*kapi.NamespaceList)), nil
}

func (s *REST) Watch(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	if ctx == nil {
		return nil, fmt.Errorf("Context is nil")
	}
	userInfo, exists := kapi.UserFrom(ctx)
	if !exists {
		return nil, fmt.Errorf("no user")
	}

	includeAllExistingProjects := (options != nil) && options.ResourceVersion == "0"

	allowedNamespaces, err := scope.ScopesToVisibleNamespaces(userInfo.GetExtra()[authorizationapi.ScopesKey], s.authCache.GetClusterPolicyLister().ClusterPolicies())
	if err != nil {
		return nil, err
	}

	watcher := projectauth.NewUserProjectWatcher(userInfo, allowedNamespaces, s.projectCache, s.authCache, includeAllExistingProjects)
	s.authCache.AddWatcher(watcher)

	go watcher.Watch()
	return watcher, nil
}

var _ = rest.Getter(&REST{})

// Get retrieves a Project by name
func (s *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	namespace, err := s.client.Get(name)
	if err != nil {
		return nil, err
	}
	return projectutil.ConvertNamespace(namespace), nil
}

var _ = rest.Creater(&REST{})

// Create registers the given Project.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	project, ok := obj.(*api.Project)
	if !ok {
		return nil, fmt.Errorf("not a project: %#v", obj)
	}
	kapi.FillObjectMetaSystemFields(ctx, &project.ObjectMeta)
	s.createStrategy.PrepareForCreate(ctx, obj)
	if errs := s.createStrategy.Validate(ctx, obj); len(errs) > 0 {
		return nil, kerrors.NewInvalid(projectapi.Kind("Project"), project.Name, errs)
	}
	namespace, err := s.client.Create(projectutil.ConvertProject(project))
	if err != nil {
		return nil, err
	}
	return projectutil.ConvertNamespace(namespace), nil
}

var _ = rest.Updater(&REST{})

func (s *REST) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	oldObj, err := s.Get(ctx, name)
	if err != nil {
		return nil, false, err
	}

	obj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}

	project, ok := obj.(*api.Project)
	if !ok {
		return nil, false, fmt.Errorf("not a project: %#v", obj)
	}

	s.updateStrategy.PrepareForUpdate(ctx, obj, oldObj)
	if errs := s.updateStrategy.ValidateUpdate(ctx, obj, oldObj); len(errs) > 0 {
		// If the user has the ability to update the namespace directly, they can circumvent most of the
		// project validation code.  Thus, as a convenience, if we see that the validation errors are all in
		// the project's immutable fields, we try to impersonate the user and update the namespace directly.
		if containsOnlyImmutableProjectFieldErrors(errs) {
			if namespace, ok := s.tryImpersonatingNamespaceUpdate(project, ctx); ok {
				return projectutil.ConvertNamespace(namespace), false, nil
			}
		}
		return nil, false, kerrors.NewInvalid(projectapi.Kind("Project"), project.Name, errs)
	}

	namespace, err := s.client.Update(projectutil.ConvertProject(project))
	if err != nil {
		return nil, false, err
	}

	return projectutil.ConvertNamespace(namespace), false, nil
}

func (s *REST) tryImpersonatingNamespaceUpdate(project *api.Project, ctx kapi.Context) (*kapi.Namespace, bool) {
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return nil, false
	}

	impersonatingClient, err := authclient.NewImpersonatingKubernetesClientset(user, s.config)
	if err != nil {
		return nil, false
	}

	namespace, err := impersonatingClient.Core().Namespaces().Update(projectutil.ConvertProject(project))
	if err != nil {
		return nil, false
	}

	return namespace, true // There was no error updating the namespace so the user has the rights to do so
}

func containsOnlyImmutableProjectFieldErrors(errs field.ErrorList) bool {
	for _, err := range errs {
		if err.Type != field.ErrorTypeInvalid || err.Detail != validation.ImmutableProjectFieldError {
			return false
		}
	}
	return true
}

var _ = rest.Deleter(&REST{})

// Delete deletes a Project specified by its name
func (s *REST) Delete(ctx kapi.Context, name string) (runtime.Object, error) {
	return &unversioned.Status{Status: unversioned.StatusSuccess}, s.client.Delete(name, nil)
}

// decoratorFunc can mutate the provided object prior to being returned.
type decoratorFunc func(obj runtime.Object) error

// filterList filters any list object that conforms to the api conventions,
// provided that 'm' works with the concrete type of list. d is an optional
// decorator for the returned functions. Only matching items are decorated.
func filterList(list runtime.Object, m kstorage.SelectionPredicate, d decoratorFunc) (filtered runtime.Object, err error) {
	// TODO: push a matcher down into tools.etcdHelper to avoid all this
	// nonsense. This is a lot of unnecessary copies.
	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, err
	}
	var filteredItems []runtime.Object
	for _, obj := range items {
		match, err := m.Matches(obj)
		if err != nil {
			return nil, err
		}
		if match {
			if d != nil {
				if err := d(obj); err != nil {
					return nil, err
				}
			}
			filteredItems = append(filteredItems, obj)
		}
	}
	err = meta.SetList(list, filteredItems)
	if err != nil {
		return nil, err
	}
	return list, nil
}
