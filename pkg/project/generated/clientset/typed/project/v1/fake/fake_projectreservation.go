package fake

import (
	project_v1 "github.com/openshift/origin/pkg/project/apis/project/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeProjectReservations implements ProjectReservationInterface
type FakeProjectReservations struct {
	Fake *FakeProjectV1
}

var projectreservationsResource = schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projectreservations"}

var projectreservationsKind = schema.GroupVersionKind{Group: "project.openshift.io", Version: "v1", Kind: "ProjectReservation"}

// Get takes name of the projectReservation, and returns the corresponding projectReservation object, and an error if there is any.
func (c *FakeProjectReservations) Get(name string, options v1.GetOptions) (result *project_v1.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(projectreservationsResource, name), &project_v1.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project_v1.ProjectReservation), err
}

// List takes label and field selectors, and returns the list of ProjectReservations that match those selectors.
func (c *FakeProjectReservations) List(opts v1.ListOptions) (result *project_v1.ProjectReservationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(projectreservationsResource, projectreservationsKind, opts), &project_v1.ProjectReservationList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &project_v1.ProjectReservationList{}
	for _, item := range obj.(*project_v1.ProjectReservationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested projectReservations.
func (c *FakeProjectReservations) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(projectreservationsResource, opts))
}

// Create takes the representation of a projectReservation and creates it.  Returns the server's representation of the projectReservation, and an error, if there is any.
func (c *FakeProjectReservations) Create(projectReservation *project_v1.ProjectReservation) (result *project_v1.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(projectreservationsResource, projectReservation), &project_v1.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project_v1.ProjectReservation), err
}

// Update takes the representation of a projectReservation and updates it. Returns the server's representation of the projectReservation, and an error, if there is any.
func (c *FakeProjectReservations) Update(projectReservation *project_v1.ProjectReservation) (result *project_v1.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(projectreservationsResource, projectReservation), &project_v1.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project_v1.ProjectReservation), err
}

// Delete takes name of the projectReservation and deletes it. Returns an error if one occurs.
func (c *FakeProjectReservations) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(projectreservationsResource, name), &project_v1.ProjectReservation{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeProjectReservations) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(projectreservationsResource, listOptions)

	_, err := c.Fake.Invokes(action, &project_v1.ProjectReservationList{})
	return err
}

// Patch applies the patch and returns the patched projectReservation.
func (c *FakeProjectReservations) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *project_v1.ProjectReservation, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(projectreservationsResource, name, data, subresources...), &project_v1.ProjectReservation{})
	if obj == nil {
		return nil, err
	}
	return obj.(*project_v1.ProjectReservation), err
}
