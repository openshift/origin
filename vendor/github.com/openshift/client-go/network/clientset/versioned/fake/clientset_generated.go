package fake

import (
	clientset "github.com/openshift/client-go/network/clientset/versioned"
	networkv1 "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1"
	fakenetworkv1 "github.com/openshift/client-go/network/clientset/versioned/typed/network/v1/fake"
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

	return &Clientset{fakePtr, &fakediscovery.FakeDiscovery{Fake: &fakePtr}}
}

// Clientset implements clientset.Interface. Meant to be embedded into a
// struct to get a default implementation. This makes faking out just the method
// you want to test easier.
type Clientset struct {
	testing.Fake
	discovery *fakediscovery.FakeDiscovery
}

func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return c.discovery
}

var _ clientset.Interface = &Clientset{}

// NetworkV1 retrieves the NetworkV1Client
func (c *Clientset) NetworkV1() networkv1.NetworkV1Interface {
	return &fakenetworkv1.FakeNetworkV1{Fake: &c.Fake}
}

// Network retrieves the NetworkV1Client
func (c *Clientset) Network() networkv1.NetworkV1Interface {
	return &fakenetworkv1.FakeNetworkV1{Fake: &c.Fake}
}
