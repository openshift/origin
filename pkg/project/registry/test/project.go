package test

import (
	"sync"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
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

func (r *ProjectRegistry) ListProjects(ctx kapi.Context, selector labels.Selector) (*api.ProjectList, error) {
	r.Lock()
	defer r.Unlock()

	return r.Projects, r.Err
}

func (r *ProjectRegistry) GetProject(ctx kapi.Context, id string) (*api.Project, error) {
	r.Lock()
	defer r.Unlock()

	return r.Project, r.Err
}

func (r *ProjectRegistry) CreateProject(ctx kapi.Context, project *api.Project) error {
	r.Lock()
	defer r.Unlock()

	r.Project = project
	return r.Err
}

func (r *ProjectRegistry) UpdateProject(ctx kapi.Context, project *api.Project) error {
	r.Lock()
	defer r.Unlock()

	r.Project = project
	return r.Err
}

func (r *ProjectRegistry) DeleteProject(ctx kapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()

	return r.Err
}
