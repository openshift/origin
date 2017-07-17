package fake

import (
	v1 "github.com/openshift/origin/pkg/security/apis/security/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeSecurityContextConstraints implements SecurityContextConstraintsInterface
type FakeSecurityContextConstraints struct {
	Fake *FakeSecurityV1
}

var securitycontextconstraintsResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraints"}

var securitycontextconstraintsKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "v1", Kind: "SecurityContextConstraints"}

func (c *FakeSecurityContextConstraints) Create(securityContextConstraints *v1.SecurityContextConstraints) (result *v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(securitycontextconstraintsResource, securityContextConstraints), &v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Update(securityContextConstraints *v1.SecurityContextConstraints) (result *v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(securitycontextconstraintsResource, securityContextConstraints), &v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(securitycontextconstraintsResource, name), &v1.SecurityContextConstraints{})
	return err
}

func (c *FakeSecurityContextConstraints) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(securitycontextconstraintsResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1.SecurityContextConstraintsList{})
	return err
}

func (c *FakeSecurityContextConstraints) Get(name string, options meta_v1.GetOptions) (result *v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(securitycontextconstraintsResource, name), &v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.SecurityContextConstraints), err
}

func (c *FakeSecurityContextConstraints) List(opts meta_v1.ListOptions) (result *v1.SecurityContextConstraintsList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(securitycontextconstraintsResource, securitycontextconstraintsKind, opts), &v1.SecurityContextConstraintsList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.SecurityContextConstraintsList{}
	for _, item := range obj.(*v1.SecurityContextConstraintsList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested securityContextConstraints.
func (c *FakeSecurityContextConstraints) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(securitycontextconstraintsResource, opts))
}

// Patch applies the patch and returns the patched securityContextConstraints.
func (c *FakeSecurityContextConstraints) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(securitycontextconstraintsResource, name, data, subresources...), &v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.SecurityContextConstraints), err
}
