package proxy

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/project/api"
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

// NewREST returns a RESTStorage object that will work against Project resources
func NewREST(client kclient.NamespaceInterface, lister projectauth.Lister) *REST {
	return &REST{
		client:         client,
		lister:         lister,
		createStrategy: projectregistry.Strategy,
		updateStrategy: projectregistry.Strategy,
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
	namespace.Annotations["displayName"] = project.Annotations["displayName"]
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
	return convertNamespaceList(namespaceList), nil
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

// Delete deletes a Project specified by its name
func (s *REST) Delete(ctx kapi.Context, name string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, s.client.Delete(name)
}
