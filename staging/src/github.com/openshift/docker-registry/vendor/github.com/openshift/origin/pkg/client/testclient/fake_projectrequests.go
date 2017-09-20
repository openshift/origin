package testclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgotesting "k8s.io/client-go/testing"

	projectapi "github.com/openshift/origin/pkg/project/apis/project"
)

// FakeProjectRequests implements ProjectInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeProjectRequests struct {
	Fake *Fake
}

var newProjectsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "newprojects"}
var newProjectsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "NewProject"}

func (c *FakeProjectRequests) List(opts metav1.ListOptions) (*metav1.Status, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(newProjectsResource, newProjectsKind, opts), &metav1.Status{})
	if obj == nil {
		return nil, err
	}

	return obj.(*metav1.Status), err
}

func (c *FakeProjectRequests) Create(inObj *projectapi.ProjectRequest) (*projectapi.Project, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(newProjectsResource, inObj), &projectapi.Project{})
	if obj == nil {
		return nil, err
	}

	return obj.(*projectapi.Project), err
}
