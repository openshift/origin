package test

import (
	"sync"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/project/api"
)

type ProjectRegistry struct {
	Err      error
	Project  *api.Project
	Projects *api.ProjectList
	sync.Mutex
}

func NewProjectRegistry() *ProjectRegistry {
	return &ProjectRegistry{}
}

func (r *ProjectRegistry) ListProjects(ctx kubeapi.Context, selector labels.Selector) (*api.ProjectList, error) {
	r.Lock()
	defer r.Unlock()

	return r.Projects, r.Err
}

func (r *ProjectRegistry) GetProject(ctx kubeapi.Context, id string) (*api.Project, error) {
	r.Lock()
	defer r.Unlock()

	return r.Project, r.Err
}

func (r *ProjectRegistry) CreateProject(ctx kubeapi.Context, project *api.Project) error {
	r.Lock()
	defer r.Unlock()

	r.Project = project
	return r.Err
}

func (r *ProjectRegistry) UpdateProject(ctx kubeapi.Context, project *api.Project) error {
	r.Lock()
	defer r.Unlock()

	r.Project = project
	return r.Err
}

func (r *ProjectRegistry) DeleteProject(ctx kubeapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}
