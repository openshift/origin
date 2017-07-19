package fake

import (
	v1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeProjects implements ProjectResourceInterface
type FakeProjects struct {
	Fake *FakeProjectV1
}

var projectsResource = schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projects"}

var projectsKind = schema.GroupVersionKind{Group: "project.openshift.io", Version: "v1", Kind: "Project"}

func (c *FakeProjects) Create(projectResource *v1.Project) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(projectsResource, projectResource), &v1.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) Update(projectResource *v1.Project) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(projectsResource, projectResource), &v1.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) UpdateStatus(projectResource *v1.Project) (*v1.Project, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(projectsResource, "status", projectResource), &v1.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(projectsResource, name), &v1.Project{})
	return err
}

func (c *FakeProjects) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(projectsResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ProjectList{})
	return err
}

func (c *FakeProjects) Get(name string, options meta_v1.GetOptions) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(projectsResource, name), &v1.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) List(opts meta_v1.ListOptions) (result *v1.ProjectList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(projectsResource, projectsKind, opts), &v1.ProjectList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ProjectList{}
	for _, item := range obj.(*v1.ProjectList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested projects.
func (c *FakeProjects) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(projectsResource, opts))
}

// Patch applies the patch and returns the patched projectResource.
func (c *FakeProjects) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(projectsResource, name, data, subresources...), &v1.Project{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}
