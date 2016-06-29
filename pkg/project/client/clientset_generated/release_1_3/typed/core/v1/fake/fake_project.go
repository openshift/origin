package fake

import (
	v1 "github.com/openshift/origin/pkg/project/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeProjects implements ProjectInterface
type FakeProjects struct {
	Fake *FakeCore
	ns   string
}

var projectsResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "projects"}

func (c *FakeProjects) Create(project *v1.Project) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(projectsResource, c.ns, project), &v1.Project{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) Update(project *v1.Project) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(projectsResource, c.ns, project), &v1.Project{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) UpdateStatus(project *v1.Project) (*v1.Project, error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateSubresourceAction(projectsResource, "status", c.ns, project), &v1.Project{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(projectsResource, c.ns, name), &v1.Project{})

	return err
}

func (c *FakeProjects) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(projectsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ProjectList{})
	return err
}

func (c *FakeProjects) Get(name string) (result *v1.Project, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(projectsResource, c.ns, name), &v1.Project{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Project), err
}

func (c *FakeProjects) List(opts api.ListOptions) (result *v1.ProjectList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(projectsResource, c.ns, opts), &v1.ProjectList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
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
func (c *FakeProjects) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(projectsResource, c.ns, opts))

}
