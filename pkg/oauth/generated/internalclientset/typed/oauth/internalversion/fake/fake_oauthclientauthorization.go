package fake

import (
	oauth "github.com/openshift/origin/pkg/oauth/apis/oauth"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeOAuthClientAuthorizations implements OAuthClientAuthorizationInterface
type FakeOAuthClientAuthorizations struct {
	Fake *FakeOauth
}

var oauthclientauthorizationsResource = schema.GroupVersionResource{Group: "oauth.openshift.io", Version: "", Resource: "oauthclientauthorizations"}

var oauthclientauthorizationsKind = schema.GroupVersionKind{Group: "oauth.openshift.io", Version: "", Kind: "OAuthClientAuthorization"}

// Get takes name of the oAuthClientAuthorization, and returns the corresponding oAuthClientAuthorization object, and an error if there is any.
func (c *FakeOAuthClientAuthorizations) Get(name string, options v1.GetOptions) (result *oauth.OAuthClientAuthorization, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(oauthclientauthorizationsResource, name), &oauth.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClientAuthorization), err
}

// List takes label and field selectors, and returns the list of OAuthClientAuthorizations that match those selectors.
func (c *FakeOAuthClientAuthorizations) List(opts v1.ListOptions) (result *oauth.OAuthClientAuthorizationList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(oauthclientauthorizationsResource, oauthclientauthorizationsKind, opts), &oauth.OAuthClientAuthorizationList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &oauth.OAuthClientAuthorizationList{}
	for _, item := range obj.(*oauth.OAuthClientAuthorizationList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested oAuthClientAuthorizations.
func (c *FakeOAuthClientAuthorizations) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(oauthclientauthorizationsResource, opts))
}

// Create takes the representation of a oAuthClientAuthorization and creates it.  Returns the server's representation of the oAuthClientAuthorization, and an error, if there is any.
func (c *FakeOAuthClientAuthorizations) Create(oAuthClientAuthorization *oauth.OAuthClientAuthorization) (result *oauth.OAuthClientAuthorization, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(oauthclientauthorizationsResource, oAuthClientAuthorization), &oauth.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClientAuthorization), err
}

// Update takes the representation of a oAuthClientAuthorization and updates it. Returns the server's representation of the oAuthClientAuthorization, and an error, if there is any.
func (c *FakeOAuthClientAuthorizations) Update(oAuthClientAuthorization *oauth.OAuthClientAuthorization) (result *oauth.OAuthClientAuthorization, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(oauthclientauthorizationsResource, oAuthClientAuthorization), &oauth.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClientAuthorization), err
}

// Delete takes name of the oAuthClientAuthorization and deletes it. Returns an error if one occurs.
func (c *FakeOAuthClientAuthorizations) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(oauthclientauthorizationsResource, name), &oauth.OAuthClientAuthorization{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOAuthClientAuthorizations) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(oauthclientauthorizationsResource, listOptions)

	_, err := c.Fake.Invokes(action, &oauth.OAuthClientAuthorizationList{})
	return err
}

// Patch applies the patch and returns the patched oAuthClientAuthorization.
func (c *FakeOAuthClientAuthorizations) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *oauth.OAuthClientAuthorization, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(oauthclientauthorizationsResource, name, data, subresources...), &oauth.OAuthClientAuthorization{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClientAuthorization), err
}
