package fake

import (
	security_v1 "github.com/openshift/api/security/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeSecurityContextConstraintses implements SecurityContextConstraintsInterface
type FakeSecurityContextConstraintses struct {
	Fake *FakeSecurityV1
}

var securitycontextconstraintsesResource = schema.GroupVersionResource{Group: "security.openshift.io", Version: "v1", Resource: "securitycontextconstraintses"}

var securitycontextconstraintsesKind = schema.GroupVersionKind{Group: "security.openshift.io", Version: "v1", Kind: "SecurityContextConstraints"}

// Get takes name of the securityContextConstraints, and returns the corresponding securityContextConstraints object, and an error if there is any.
func (c *FakeSecurityContextConstraintses) Get(name string, options v1.GetOptions) (result *security_v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(securitycontextconstraintsesResource, name), &security_v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security_v1.SecurityContextConstraints), err
}

// List takes label and field selectors, and returns the list of SecurityContextConstraintses that match those selectors.
func (c *FakeSecurityContextConstraintses) List(opts v1.ListOptions) (result *security_v1.SecurityContextConstraintsList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(securitycontextconstraintsesResource, securitycontextconstraintsesKind, opts), &security_v1.SecurityContextConstraintsList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &security_v1.SecurityContextConstraintsList{}
	for _, item := range obj.(*security_v1.SecurityContextConstraintsList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested securityContextConstraintses.
func (c *FakeSecurityContextConstraintses) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(securitycontextconstraintsesResource, opts))
}

// Create takes the representation of a securityContextConstraints and creates it.  Returns the server's representation of the securityContextConstraints, and an error, if there is any.
func (c *FakeSecurityContextConstraintses) Create(securityContextConstraints *security_v1.SecurityContextConstraints) (result *security_v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(securitycontextconstraintsesResource, securityContextConstraints), &security_v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security_v1.SecurityContextConstraints), err
}

// Update takes the representation of a securityContextConstraints and updates it. Returns the server's representation of the securityContextConstraints, and an error, if there is any.
func (c *FakeSecurityContextConstraintses) Update(securityContextConstraints *security_v1.SecurityContextConstraints) (result *security_v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(securitycontextconstraintsesResource, securityContextConstraints), &security_v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security_v1.SecurityContextConstraints), err
}

// Delete takes name of the securityContextConstraints and deletes it. Returns an error if one occurs.
func (c *FakeSecurityContextConstraintses) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(securitycontextconstraintsesResource, name), &security_v1.SecurityContextConstraints{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSecurityContextConstraintses) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(securitycontextconstraintsesResource, listOptions)

	_, err := c.Fake.Invokes(action, &security_v1.SecurityContextConstraintsList{})
	return err
}

// Patch applies the patch and returns the patched securityContextConstraints.
func (c *FakeSecurityContextConstraintses) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *security_v1.SecurityContextConstraints, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(securitycontextconstraintsesResource, name, data, subresources...), &security_v1.SecurityContextConstraints{})
	if obj == nil {
		return nil, err
	}
	return obj.(*security_v1.SecurityContextConstraints), err
}
