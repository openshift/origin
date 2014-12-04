package client

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	projectapi "github.com/openshift/origin/pkg/project/api"
	_ "github.com/openshift/origin/pkg/user/api/v1beta1"
)

// UsersInterface has methods to work with User resources in a namespace
type ProjectsInterface interface {
	Projects() ProjectInterface
}

// UserInterface exposes methods on user resources.
type ProjectInterface interface {
	Get(name string) (*projectapi.Project, error)
	List(label, field labels.Selector) (*projectapi.ProjectList, error)
}

type projects struct {
	r *Client
}

// newUsers returns a users
func newProjects(c *Client) *projects {
	return &projects{
		r: c,
	}
}

// Get returns information about a particular user or an error
func (c *projects) Get(name string) (result *projectapi.Project, err error) {
	result = &projectapi.Project{}
	err = c.r.Get().Path("projects").Path(name).Do().Into(result)
	return
}

func (c *projects) List(label, field labels.Selector) (result *projectapi.ProjectList, err error) {
	result = &projectapi.ProjectList{}
	err = c.r.Get().
		Path("projects").
		SelectorParam("labels", label).
		SelectorParam("fields", field).
		Do().
		Into(result)
	return
}
