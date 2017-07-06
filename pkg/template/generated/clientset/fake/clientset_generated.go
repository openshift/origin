package fake

import (
	clientset "github.com/openshift/origin/pkg/template/generated/clientset"
	templatev1 "github.com/openshift/origin/pkg/template/generated/clientset/typed/template/v1"
	faketemplatev1 "github.com/openshift/origin/pkg/template/generated/clientset/typed/template/v1/fake"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/testing"
)

// NewSimpleClientset returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
func NewSimpleClientset(objects ...runtime.Object) *Clientset {
	o := testing.NewObjectTracker(scheme, codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	fakePtr := testing.Fake{}
	fakePtr.AddReactor("*", "*", testing.ObjectReaction(o))

	fakePtr.AddWatchReactor("*", testing.DefaultWatchReactor(watch.NewFake(), nil))

	return &Clientset{fakePtr}
}

// Clientset implements clientset.Interface. Meant to be embedded into a
// struct to get a default implementation. This makes faking out just the method
// you want to test easier.
type Clientset struct {
	testing.Fake
}

func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return &fakediscovery.FakeDiscovery{Fake: &c.Fake}
}

var _ clientset.Interface = &Clientset{}

// TemplateV1 retrieves the TemplateV1Client
func (c *Clientset) TemplateV1() templatev1.TemplateV1Interface {
	return &faketemplatev1.FakeTemplateV1{Fake: &c.Fake}
}

// Template retrieves the TemplateV1Client
func (c *Clientset) Template() templatev1.TemplateV1Interface {
	return &faketemplatev1.FakeTemplateV1{Fake: &c.Fake}
}
