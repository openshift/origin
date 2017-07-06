package fake

import (
	security "github.com/openshift/origin/pkg/security/apis/security"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeSecurityContextConstraints implements SecurityContextConstraintsInterface
type FakeSecurityContextConstraints struct {
	Fake *FakeSecurity
}

var securitycontextconstraintsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "", Resource: "securitycontextconstraints"}

var securitycontextconstraintsKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "", Kind: "SecurityContextConstraints"}

func (c *FakeSecurityContextConstraints) Create(securityContextConstraints *security.SecurityContextConstraints) (result *security.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(securitycontextconstraintsResource, securityContextConstraints), &security.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Update(securityContextConstraints *security.SecurityContextConstraints) (result *security.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(securitycontextconstraintsResource, securityContextConstraints), &security.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(securitycontextconstraintsResource, name), &security.SecurityContextConstraints{})
	return err
}

func (c *FakeSecurityContextConstraints) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(securitycontextconstraintsResource, listOptions)

	_, err := c.Fake.Invokes(action, &security.SecurityContextConstraintsList{})
	return err
}

func (c *FakeSecurityContextConstraints) Get(name string, options v1.GetOptions) (result *security.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(securitycontextconstraintsResource, name), &security.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) List(opts v1.ListOptions) (result *security.SecurityContextConstraintsList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(securitycontextconstraintsResource, securitycontextconstraintsKind, opts), &security.SecurityContextConstraintsList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &security.SecurityContextConstraintsList{}
	for _, item := range obj.(*security.SecurityContextConstraintsList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested securityContextConstraints.
func (c *FakeSecurityContextConstraints) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(securitycontextconstraintsResource, opts))
}

// Patch applies the patch and returns the patched securityContextConstraints.
func (c *FakeSecurityContextConstraints) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *security.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(securitycontextconstraintsResource, name, data, subresources...), &security.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security.SecurityContextConstraints), err
}
