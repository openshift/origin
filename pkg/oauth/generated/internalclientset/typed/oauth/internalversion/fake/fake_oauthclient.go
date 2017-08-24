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

// FakeOAuthClients implements OAuthClientInterface
type FakeOAuthClients struct {
	Fake *FakeOauth
}

var oauthclientsResource = schema.GroupVersionResource{Group: "oauth.openshift.io", Version: "", Resource: "oauthclients"}

var oauthclientsKind = schema.GroupVersionKind{Group: "oauth.openshift.io", Version: "", Kind: "OAuthClient"}

// Get takes name of the oAuthClient, and returns the corresponding oAuthClient object, and an error if there is any.
func (c *FakeOAuthClients) Get(name string, options v1.GetOptions) (result *oauth.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(oauthclientsResource, name), &oauth.OAuthClient{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClient), err
}

// List takes label and field selectors, and returns the list of OAuthClients that match those selectors.
func (c *FakeOAuthClients) List(opts v1.ListOptions) (result *oauth.OAuthClientList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(oauthclientsResource, oauthclientsKind, opts), &oauth.OAuthClientList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &oauth.OAuthClientList{}
	for _, item := range obj.(*oauth.OAuthClientList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested oAuthClients.
func (c *FakeOAuthClients) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(oauthclientsResource, opts))
}

// Create takes the representation of a oAuthClient and creates it.  Returns the server's representation of the oAuthClient, and an error, if there is any.
func (c *FakeOAuthClients) Create(oAuthClient *oauth.OAuthClient) (result *oauth.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(oauthclientsResource, oAuthClient), &oauth.OAuthClient{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClient), err
}

// Update takes the representation of a oAuthClient and updates it. Returns the server's representation of the oAuthClient, and an error, if there is any.
func (c *FakeOAuthClients) Update(oAuthClient *oauth.OAuthClient) (result *oauth.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(oauthclientsResource, oAuthClient), &oauth.OAuthClient{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClient), err
}

// Delete takes name of the oAuthClient and deletes it. Returns an error if one occurs.
func (c *FakeOAuthClients) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(oauthclientsResource, name), &oauth.OAuthClient{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeOAuthClients) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(oauthclientsResource, listOptions)

	_, err := c.Fake.Invokes(action, &oauth.OAuthClientList{})
	return err
}

// Patch applies the patch and returns the patched oAuthClient.
func (c *FakeOAuthClients) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *oauth.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(oauthclientsResource, name, data, subresources...), &oauth.OAuthClient{})
	if obj == nil {
		return nil, err
	}
	return obj.(*oauth.OAuthClient), err
}
