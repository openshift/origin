package legacyclient

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"

	otestclient "github.com/openshift/origin/pkg/client/testclient"
	securityapi "github.com/openshift/origin/pkg/security/api"
)

// NewSimpleFake returns a client that will respond with the provided objects
func NewSimpleFake(objects ...runtime.Object) *FakeSecurityContextContstraint {
	o := clientgotesting.NewObjectTracker(kapi.Registry, kapi.Scheme, kapi.Codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	fakeClient := &otestclient.Fake{}
	fakeClient.AddReactor("*", "*", clientgotesting.ObjectReaction(o, kapi.Registry.RESTMapper()))

	fakeClient.AddWatchReactor("*", clientgotesting.DefaultWatchReactor(watch.NewFake(), nil))

	return FakeSecurityContextContstraint{Fake: fakeClient}
}

type FakeSecurityContextContstraint struct {
	Fake *otestclient.Fake
}

var sccResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "securitycontextconstraints"}

func (c *FakeSecurityContextContstraint) Get(name string, options metav1.GetOptions) (*securityapi.ClusterPolicy, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(sccResource, name), &securityapi.ClusterPolicy{})
	if obj == nil {
		return nil, err
	}

	return obj.(*securityapi.ClusterPolicy), err
}

func (c *FakeSecurityContextContstraint) List(opts metav1.ListOptions) (*securityapi.ClusterPolicyList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(sccResource, opts), &securityapi.ClusterPolicyList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*securityapi.ClusterPolicyList), err
}

func (c *FakeSecurityContextContstraint) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(sccResource, name), &securityapi.ClusterPolicy{})
	return err
}

func (c *FakeSecurityContextContstraint) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(sccResource, opts))
}
