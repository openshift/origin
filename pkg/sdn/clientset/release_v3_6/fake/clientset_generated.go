package fake

import (
	clientset "github.com/openshift/origin/pkg/sdn/clientset/release_v3_6"
	v1sdn "github.com/openshift/origin/pkg/sdn/clientset/release_v3_6/typed/sdn/v1"
	fakev1sdn "github.com/openshift/origin/pkg/sdn/clientset/release_v3_6/typed/sdn/v1/fake"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/client/typed/discovery"
	fakediscovery "k8s.io/kubernetes/pkg/client/typed/discovery/fake"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

// NewSimpleClientset returns a clientset that will respond with the provided objects.
// It's backed by a very simple object tracker that processes creates, updates and deletions as-is,
// without applying any validations and/or defaults. It shouldn't be considered a replacement
// for a real clientset and is mostly useful in simple unit tests.
func NewSimpleClientset(objects ...runtime.Object) *Clientset {
	o := core.NewObjectTracker(api.Scheme, api.Codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	fakePtr := core.Fake{}
	fakePtr.AddReactor("*", "*", core.ObjectReaction(o, registered.RESTMapper()))

	fakePtr.AddWatchReactor("*", core.DefaultWatchReactor(watch.NewFake(), nil))

	return &Clientset{fakePtr}
}

// Clientset implements clientset.Interface. Meant to be embedded into a
// struct to get a default implementation. This makes faking out just the method
// you want to test easier.
type Clientset struct {
	core.Fake
}

func (c *Clientset) Discovery() discovery.DiscoveryInterface {
	return &fakediscovery.FakeDiscovery{Fake: &c.Fake}
}

var _ clientset.Interface = &Clientset{}

// SdnV1 retrieves the SdnV1Client
func (c *Clientset) SdnV1() v1sdn.SdnV1Interface {
	return &fakev1sdn.FakeSdnV1{Fake: &c.Fake}
}

// Sdn retrieves the SdnV1Client
func (c *Clientset) Sdn() v1sdn.SdnV1Interface {
	return &fakev1sdn.FakeSdnV1{Fake: &c.Fake}
}
