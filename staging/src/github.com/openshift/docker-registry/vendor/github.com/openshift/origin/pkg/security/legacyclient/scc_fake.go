package legacyclient

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"

	otestclient "github.com/openshift/origin/pkg/client/testclient"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

// NewSimpleFake returns a client that will respond with the provided objects
func NewSimpleFake(objects ...runtime.Object) *FakeSecurityContextContstraint {
	o := clientgotesting.NewObjectTracker(kapi.Scheme, kapi.Codecs.UniversalDecoder())
	for _, obj := range objects {
		if err := o.Add(obj); err != nil {
			panic(err)
		}
	}

	fakeClient := &otestclient.Fake{}
	fakeClient.AddReactor("*", "*", clientgotesting.ObjectReaction(o))

	fakeClient.AddWatchReactor("*", clientgotesting.DefaultWatchReactor(watch.NewFake(), nil))

	return &FakeSecurityContextContstraint{Fake: fakeClient}
}

type FakeSecurityContextContstraint struct {
	Fake *otestclient.Fake
}

var sccResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "securitycontextconstraints"}
var sccKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "SecurityContextConstraints"}

func (c *FakeSecurityContextContstraint) Get(name string, options metav1.GetOptions) (*securityapi.SecurityContextConstraints, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootGetAction(sccResource, name), &securityapi.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}

	return obj.(*securityapi.SecurityContextConstraints), err
}

func (c *FakeSecurityContextContstraint) List(opts metav1.ListOptions) (*securityapi.SecurityContextConstraintsList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootListAction(sccResource, sccKind, opts), &securityapi.SecurityContextConstraintsList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*securityapi.SecurityContextConstraintsList), err
}

func (c *FakeSecurityContextContstraint) Create(inObj *securityapi.SecurityContextConstraints) (*securityapi.SecurityContextConstraints, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootCreateAction(sccResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*securityapi.SecurityContextConstraints), err
}

func (c *FakeSecurityContextContstraint) Update(inObj *securityapi.SecurityContextConstraints) (*securityapi.SecurityContextConstraints, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewRootUpdateAction(sccResource, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*securityapi.SecurityContextConstraints), err
}

func (c *FakeSecurityContextContstraint) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewRootDeleteAction(sccResource, name), &securityapi.SecurityContextConstraints{})
	return err
}

func (c *FakeSecurityContextContstraint) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewRootWatchAction(sccResource, opts))
}
