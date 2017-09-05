package fake

import (
	network "github.com/openshift/origin/pkg/network/apis/network"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeNetNamespaces implements NetNamespaceInterface
type FakeNetNamespaces struct {
	Fake *FakeNetwork
}

var netnamespacesResource = schema.GroupVersionResource{Group: "network.openshift.io", Version: "", Resource: "netnamespaces"}

var netnamespacesKind = schema.GroupVersionKind{Group: "network.openshift.io", Version: "", Kind: "NetNamespace"}

// Get takes name of the netNamespace, and returns the corresponding netNamespace object, and an error if there is any.
func (c *FakeNetNamespaces) Get(name string, options v1.GetOptions) (result *network.NetNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(netnamespacesResource, name), &network.NetNamespace{})
	if obj == nil {
		return nil, err
	}
	return obj.(*network.NetNamespace), err
}

// List takes label and field selectors, and returns the list of NetNamespaces that match those selectors.
func (c *FakeNetNamespaces) List(opts v1.ListOptions) (result *network.NetNamespaceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(netnamespacesResource, netnamespacesKind, opts), &network.NetNamespaceList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &network.NetNamespaceList{}
	for _, item := range obj.(*network.NetNamespaceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested netNamespaces.
func (c *FakeNetNamespaces) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(netnamespacesResource, opts))
}

// Create takes the representation of a netNamespace and creates it.  Returns the server's representation of the netNamespace, and an error, if there is any.
func (c *FakeNetNamespaces) Create(netNamespace *network.NetNamespace) (result *network.NetNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(netnamespacesResource, netNamespace), &network.NetNamespace{})
	if obj == nil {
		return nil, err
	}
	return obj.(*network.NetNamespace), err
}

// Update takes the representation of a netNamespace and updates it. Returns the server's representation of the netNamespace, and an error, if there is any.
func (c *FakeNetNamespaces) Update(netNamespace *network.NetNamespace) (result *network.NetNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(netnamespacesResource, netNamespace), &network.NetNamespace{})
	if obj == nil {
		return nil, err
	}
	return obj.(*network.NetNamespace), err
}

// Delete takes name of the netNamespace and deletes it. Returns an error if one occurs.
func (c *FakeNetNamespaces) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(netnamespacesResource, name), &network.NetNamespace{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeNetNamespaces) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(netnamespacesResource, listOptions)

	_, err := c.Fake.Invokes(action, &network.NetNamespaceList{})
	return err
}

// Patch applies the patch and returns the patched netNamespace.
func (c *FakeNetNamespaces) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *network.NetNamespace, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(netnamespacesResource, name, data, subresources...), &network.NetNamespace{})
	if obj == nil {
		return nil, err
	}
	return obj.(*network.NetNamespace), err
}
