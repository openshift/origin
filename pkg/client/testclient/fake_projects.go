package testclient

import (
	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/watch"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

// FakeProjects implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjects struct {
	Fake *Fake
}

func (c *FakeProjects) Get(name string) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootGetAction("projects", name), &projectapi.Project{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) List(opts kapi.ListOptions) (*projectapi.ProjectList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootListAction("projects", opts), &projectapi.ProjectList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.ProjectList), err
}

func (c *FakeProjects) Create(inObj *projectapi.Project) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootCreateAction("projects", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) Update(inObj *projectapi.Project) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewRootUpdateAction("projects", inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}

func (c *FakeProjects) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewRootDeleteAction("projects", name), &projectapi.Project{})
	return err
}

func (c *FakeProjects) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewRootWatchAction("projects", opts))
}
