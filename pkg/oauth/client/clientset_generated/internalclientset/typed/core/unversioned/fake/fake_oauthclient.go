package fake

import (
	api "github.com/openshift/origin/pkg/oauth/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	labels "k8s.io/kubernetes/pkg/labels"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakeOAuthClients implements OAuthClientInterface
type FakeOAuthClients struct {
	Fake *FakeCore
	ns   string
}

var oauthclientsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "oauthclients"}

func (c *FakeOAuthClients) Create(oAuthClient *api.OAuthClient) (result *api.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(oauthclientsResource, c.ns, oAuthClient), &api.OAuthClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.OAuthClient), err
}

func (c *FakeOAuthClients) Update(oAuthClient *api.OAuthClient) (result *api.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(oauthclientsResource, c.ns, oAuthClient), &api.OAuthClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.OAuthClient), err
}

func (c *FakeOAuthClients) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(oauthclientsResource, c.ns, name), &api.OAuthClient{})

	return err
}

func (c *FakeOAuthClients) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(oauthclientsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.OAuthClientList{})
	return err
}

func (c *FakeOAuthClients) Get(name string) (result *api.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(oauthclientsResource, c.ns, name), &api.OAuthClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.OAuthClient), err
}

func (c *FakeOAuthClients) List(opts pkg_api.ListOptions) (result *api.OAuthClientList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(oauthclientsResource, c.ns, opts), &api.OAuthClientList{})

	if obj == nil {
		return nil, err
	}

	label := opts.LabelSelector
	if label == nil {
		label = labels.Everything()
	}
	list := &api.OAuthClientList{}
	for _, item := range obj.(*api.OAuthClientList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested oAuthClients.
func (c *FakeOAuthClients) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(oauthclientsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched oAuthClient.
func (c *FakeOAuthClients) Patch(name string, pt pkg_api.PatchType, data []byte, subresources ...string) (result *api.OAuthClient, err error) {
	obj, err := c.Fake.
		Invokes(core.NewPatchSubresourceAction(oauthclientsResource, c.ns, name, data, subresources...), &api.OAuthClient{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.OAuthClient), err
}
