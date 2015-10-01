package testclient

import (
	"fmt"
	"net/url"

	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
)

// FakeBuildConfigs implements BuildConfigInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildConfigs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeBuildConfigs) Get(name string) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewGetAction("buildconfigs", c.Namespace, name), &buildapi.BuildConfig{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) List(label labels.Selector, field fields.Selector) (*buildapi.BuildConfigList, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewListAction("buildconfigs", c.Namespace, label, field), &buildapi.BuildConfigList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfigList), err
}

func (c *FakeBuildConfigs) Create(inObj *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewCreateAction("buildconfigs", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) Update(inObj *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(ktestclient.NewUpdateAction("buildconfigs", c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) Delete(name string) error {
	_, err := c.Fake.Invokes(ktestclient.NewDeleteAction("buildconfigs", c.Namespace, name), &buildapi.BuildConfig{})
	return err
}

func (c *FakeBuildConfigs) Watch(label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return c.Fake.InvokesWatch(ktestclient.NewWatchAction("buildconfigs", c.Namespace, label, field, resourceVersion))
}

func (c *FakeBuildConfigs) WebHookURL(name string, trigger *buildapi.BuildTriggerPolicy) (*url.URL, error) {
	switch {
	case trigger.GenericWebHook != nil:
		return url.Parse(fmt.Sprintf("http://localhost/buildConfigHooks/%s/%s/generic", name, trigger.GenericWebHook.Secret))
	case trigger.GitHubWebHook != nil:
		return url.Parse(fmt.Sprintf("http://localhost/buildConfigHooks/%s/%s/github", name, trigger.GitHubWebHook.Secret))
	default:
		return nil, client.ErrTriggerIsNotAWebHook
	}
}

func (c *FakeBuildConfigs) Instantiate(request *buildapi.BuildRequest) (result *buildapi.Build, err error) {
	action := ktestclient.NewCreateAction("buildconfigs", c.Namespace, request)
	action.Subresource = "instantiate"
	obj, err := c.Fake.Invokes(action, &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}
