package project

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/project/api/validation"
	projectauth "github.com/openshift/origin/pkg/project/auth"
)

type REST struct {
	client kclient.NamespaceInterface
	lister projectauth.Lister
}

// NewREST returns a RESTStorage object that will work against Project resources
func NewREST(client kclient.NamespaceInterface, lister projectauth.Lister) *REST {
	return &REST{client: client, lister: lister}
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
	displayName := namespace.Annotations["displayname"]
	return &api.Project{
		ObjectMeta:  namespace.ObjectMeta,
		DisplayName: displayName,
		Spec:        api.ProjectSpec{},
		Status: api.ProjectStatus{
			Phase: namespace.Status.Phase,
		},
	}
}

// convertProject transforms a Project into a Namespace
func convertProject(project *api.Project) *kapi.Namespace {
	namespace := &kapi.Namespace{
		ObjectMeta: project.ObjectMeta,
	}
	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}
	namespace.Annotations["displayname"] = project.DisplayName
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
		return nil, kerrors.NewForbidden("Project", "", fmt.Errorf("Unable to list projects without a user on the context"))
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
	if errs := validation.ValidateProject(project); len(errs) > 0 {
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
