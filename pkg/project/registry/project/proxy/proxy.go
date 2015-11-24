package proxy

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	authuser "k8s.io/kubernetes/pkg/auth/user"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	nsregistry "k8s.io/kubernetes/pkg/registry/namespace"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/project/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectregistry "github.com/openshift/origin/pkg/project/registry/project"
)

type REST struct {
	// client can modify Kubernetes namespaces
	client kclient.NamespaceInterface
	// lister can enumerate project lists that enforce policy
	lister projectauth.Lister
	// Allows extended behavior during creation, required
	createStrategy rest.RESTCreateStrategy
	// Allows extended behavior during updates, required
	updateStrategy rest.RESTUpdateStrategy
}

type UserRetriever interface {
	User(name string) (authuser.Info, error)
}

type UserREST struct {
	// lister can enumerate project lists that enforce policy
	lister projectauth.Lister
	users  UserRetriever
}

// NewREST returns a RESTStorage object that will work against Project resources
func NewREST(client kclient.NamespaceInterface, lister projectauth.Lister, users UserRetriever) (*REST, *UserREST) {
	return &REST{
			client:         client,
			lister:         lister,
			createStrategy: projectregistry.Strategy,
			updateStrategy: projectregistry.Strategy,
		}, &UserREST{
			lister: lister,
			users:  users,
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

// convertNamespace transforms a Namespace into a Project
func convertNamespace(namespace *kapi.Namespace) *api.Project {
	return &api.Project{
		ObjectMeta: namespace.ObjectMeta,
		Spec: api.ProjectSpec{
			Finalizers: namespace.Spec.Finalizers,
		},
		Status: api.ProjectStatus{
			Phase: namespace.Status.Phase,
		},
	}
}

// convertProject transforms a Project into a Namespace
func convertProject(project *api.Project) *kapi.Namespace {
	namespace := &kapi.Namespace{
		ObjectMeta: project.ObjectMeta,
		Spec: kapi.NamespaceSpec{
			Finalizers: project.Spec.Finalizers,
		},
		Status: kapi.NamespaceStatus{
			Phase: project.Status.Phase,
		},
	}
	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}
	namespace.Annotations[projectapi.ProjectDisplayName] = project.Annotations[projectapi.ProjectDisplayName]
	return namespace
}

// convertNamespaceList transforms a NamespaceList into a ProjectList
func convertNamespaceList(namespaceList *kapi.NamespaceList) *api.ProjectList {
	projects := &api.ProjectList{}
	for _, n := range namespaceList.Items {
		projects.Items = append(projects.Items, *convertNamespace(&n))
	}
	return projects
}

// List retrieves a list of Projects that match label.
func (s *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return nil, kerrors.NewForbidden("Project", "", fmt.Errorf("unable to list projects without a user on the context"))
	}
	namespaceList, err := s.lister.List(user)
	if err != nil {
		return nil, err
	}
	m := nsregistry.MatchNamespace(label, field)
	list, err := filterList(namespaceList, m, nil)
	if err != nil {
		return nil, err
	}
	return convertNamespaceList(list.(*kapi.NamespaceList)), nil
}

// Get retrieves a Project by name
func (s *REST) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	namespace, err := s.client.Get(name)
	if err != nil {
		return nil, err
	}
	return convertNamespace(namespace), nil
}

// Create registers the given Project.
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	project, ok := obj.(*api.Project)
	if !ok {
		return nil, fmt.Errorf("not a project: %#v", obj)
	}
	kapi.FillObjectMetaSystemFields(ctx, &project.ObjectMeta)
	s.createStrategy.PrepareForCreate(obj)
	if errs := s.createStrategy.Validate(ctx, obj); len(errs) > 0 {
		return nil, kerrors.NewInvalid("project", project.Name, errs)
	}
	namespace, err := s.client.Create(convertProject(project))
	if err != nil {
		return nil, err
	}
	return convertNamespace(namespace), nil
}

func (s *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	project, ok := obj.(*api.Project)
	if !ok {
		return nil, false, fmt.Errorf("not a project: %#v", obj)
	}

	oldObj, err := s.Get(ctx, project.Name)
	if err != nil {
		return nil, false, err
	}
	s.updateStrategy.PrepareForUpdate(obj, oldObj)
	if errs := s.updateStrategy.ValidateUpdate(ctx, obj, oldObj); len(errs) > 0 {
		return nil, false, kerrors.NewInvalid("project", project.Name, errs)
	}

	namespace, err := s.client.Update(convertProject(project))
	if err != nil {
		return nil, false, err
	}

	return convertNamespace(namespace), false, nil
}

// Delete deletes a Project specified by its name
func (s *REST) Delete(ctx kapi.Context, name string) (runtime.Object, error) {
	return &unversioned.Status{Status: unversioned.StatusSuccess}, s.client.Delete(name)
}

// decoratorFunc can mutate the provided object prior to being returned.
type decoratorFunc func(obj runtime.Object) error

// filterList filters any list object that conforms to the api conventions,
// provided that 'm' works with the concrete type of list. d is an optional
// decorator for the returned functions. Only matching items are decorated.
func filterList(list runtime.Object, m generic.Matcher, d decoratorFunc) (filtered runtime.Object, err error) {
	// TODO: push a matcher down into tools.etcdHelper to avoid all this
	// nonsense. This is a lot of unnecessary copies.
	items, err := runtime.ExtractList(list)
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
	err = runtime.SetList(list, filteredItems)
	if err != nil {
		return nil, kerrors.NewInternalError(err)
	}
	return list, nil
}

// Get retrieves the projects that the provided user has access to.
func (s *UserREST) Get(ctx kapi.Context, name string, options runtime.Object) (runtime.Object, error) {
	listOptions, _ := options.(*kapi.ListOptions)
	if listOptions == nil {
		listOptions = &kapi.ListOptions{}
	}

	var user authuser.Info
	if ctxUser, ok := kapi.UserFrom(ctx); ok && ctxUser.GetName() == name {
		user = ctxUser
	}
	if user == nil {
		retrievedUser, err := s.users.User(name)
		if err != nil {
			return nil, err
		}
		user = retrievedUser
	}

	namespaceList, err := s.lister.List(user)
	if err != nil {
		return nil, err
	}

	m := nsregistry.MatchNamespace(listOptions.LabelSelector, listOptions.FieldSelector)
	list, err := filterList(namespaceList, m, nil)
	if err != nil {
		return nil, err
	}
	return convertNamespaceList(list.(*kapi.NamespaceList)), nil
}

func (s *UserREST) New() runtime.Object {
	return &projectapi.ProjectList{}
}

func (s *UserREST) NewGetOptions() (runtime.Object, bool, string) {
	return &kapi.ListOptions{}, false, ""
}
