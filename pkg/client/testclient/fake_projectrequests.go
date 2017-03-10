package testclient

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubernetes/pkg/client/testing/core"

	projectapi "github.com/openshift/origin/pkg/project/api"
)

// FakeProjectRequests implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjectRequests struct {
	Fake *Fake
}

var newProjectsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "newprojects"}

func (c *FakeProjectRequests) List(opts metainternal.ListOptions) (*metav1.Status, error) {
	obj, err := c.Fake.Invokes(core.NewRootListAction(newProjectsResource, opts), &metav1.Status{})
	if obj == nil {
		return nil, err
	}

	return obj.(*metav1.Status), err
}

func (c *FakeProjectRequests) Create(inObj *projectapi.ProjectRequest) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(core.NewRootCreateAction(newProjectsResource, inObj), &projectapi.Project{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}
