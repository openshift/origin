package fake

import (
	api "github.com/openshift/origin/pkg/project/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeProjects implements ProjectResourceInterface
type FakeProjects struct {
	Fake *FakeProject
}

var projectsResource = schema.GroupVersionResource{Group: "project.openshift.io", Version: "", Resource: "projects"}

func (c *FakeProjects) Create(project *api.Project) (result *api.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(projectsResource, project), &api.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Project), err
}

func (c *FakeProjects) Update(project *api.Project) (result *api.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(projectsResource, project), &api.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Project), err
}

func (c *FakeProjects) UpdateStatus(project *api.Project) (*api.Project, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(projectsResource, "status", project), &api.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Project), err
}

func (c *FakeProjects) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(projectsResource, name), &api.Project{})
	return err
}

func (c *FakeProjects) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(projectsResource, listOptions)

	_, err := c.Fake.Invokes(action, &api.ProjectList{})
	return err
}

func (c *FakeProjects) Get(name string, options v1.GetOptions) (result *api.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(projectsResource, name), &api.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Project), err
}

func (c *FakeProjects) List(opts v1.ListOptions) (result *api.ProjectList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(projectsResource, opts), &api.ProjectList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &api.ProjectList{}
	for _, item := range obj.(*api.ProjectList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested projects.
func (c *FakeProjects) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(projectsResource, opts))
}

// Patch applies the patch and returns the patched project.
func (c *FakeProjects) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *api.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(projectsResource, name, data, subresources...), &api.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*api.Project), err
}
