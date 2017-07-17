package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	projectapi "github.com/openshift/origin/pkg/project/apis/project"
)

// FakeProjects implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjects struct {
	Fake *Fake
}

var projectsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "projects"}
var projectsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "Project"}

func (c *FakeProjects) Get(name string, options metav1.GetOptions) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(projectsResource, name), &projectapi.Project{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) List(opts metav1.ListOptions) (*projectapi.ProjectList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(projectsResource, projectsKind, opts), &projectapi.ProjectList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.ProjectList), err
}

func (c *FakeProjects) Create(inObj *projectapi.Project) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(projectsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) Update(inObj *projectapi.Project) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(projectsResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(projectsResource, name), &projectapi.Project{})
	return err
}

func (c *FakeProjects) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(projectsResource, opts))
}
