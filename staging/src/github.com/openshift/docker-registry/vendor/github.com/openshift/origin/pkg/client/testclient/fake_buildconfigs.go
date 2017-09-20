package testclient

import (
	"fmt"
	"io"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	clientgotesting "k8s.io/client-go/testing"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/client"
)

// FakeBuildConfigs implements BuildConfigInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildConfigs struct {
	Fake      *Fake
	Namespace string
}

var buildConfigsResource = schema.GroupVersionResource{Group: "", Version: "", Resource: "buildconfigs"}
var buildConfigsKind = schema.GroupVersionKind{Group: "", Version: "", Kind: "BuildConfig"}

func (c *FakeBuildConfigs) Get(name string, options metav1.GetOptions) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewGetAction(buildConfigsResource, c.Namespace, name), &buildapi.BuildConfig{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) List(opts metav1.ListOptions) (*buildapi.BuildConfigList, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewListAction(buildConfigsResource, buildConfigsKind, c.Namespace, opts), &buildapi.BuildConfigList{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfigList), err
}

func (c *FakeBuildConfigs) Create(inObj *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewCreateAction(buildConfigsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) Update(inObj *buildapi.BuildConfig) (*buildapi.BuildConfig, error) {
	obj, err := c.Fake.Invokes(clientgotesting.NewUpdateAction(buildConfigsResource, c.Namespace, inObj), inObj)
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.BuildConfig), err
}

func (c *FakeBuildConfigs) Delete(name string) error {
	_, err := c.Fake.Invokes(clientgotesting.NewDeleteAction(buildConfigsResource, c.Namespace, name), &buildapi.BuildConfig{})
	return err
}

func (c *FakeBuildConfigs) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.InvokesWatch(clientgotesting.NewWatchAction(buildConfigsResource, c.Namespace, opts))
}

func (c *FakeBuildConfigs) WebHookURL(name string, trigger *buildapi.BuildTriggerPolicy) (*url.URL, error) {
	switch {
	case trigger.GenericWebHook != nil:
		return url.Parse(fmt.Sprintf("http://localhost/buildConfigHooks/%s/%s/generic", name, trigger.GenericWebHook.Secret))
	case trigger.GitHubWebHook != nil:
		return url.Parse(fmt.Sprintf("http://localhost/buildConfigHooks/%s/%s/github", name, trigger.GitHubWebHook.Secret))
	case trigger.GitLabWebHook != nil:
		return url.Parse(fmt.Sprintf("http://localhost/buildConfigHooks/%s/%s/gitlab", name, trigger.GitLabWebHook.Secret))
	case trigger.BitbucketWebHook != nil:
		return url.Parse(fmt.Sprintf("http://localhost/buildConfigHooks/%s/%s/bitbucket", name, trigger.BitbucketWebHook.Secret))
	default:
		return nil, client.ErrTriggerIsNotAWebHook
	}
}

func (c *FakeBuildConfigs) Instantiate(request *buildapi.BuildRequest) (result *buildapi.Build, err error) {
	action := clientgotesting.NewCreateAction(buildapi.LegacySchemeGroupVersion.WithResource("builds"), c.Namespace, request)
	action.Subresource = "instantiate"
	obj, err := c.Fake.Invokes(action, &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}

func (c *FakeBuildConfigs) InstantiateBinary(request *buildapi.BinaryBuildRequestOptions, r io.Reader) (result *buildapi.Build, err error) {
	action := clientgotesting.NewCreateAction(buildapi.LegacySchemeGroupVersion.WithResource("builds"), c.Namespace, request)
	action.Subresource = "instantiatebinary"
	obj, err := c.Fake.Invokes(action, &buildapi.Build{})
	if obj == nil {
		return nil, err
	}

	return obj.(*buildapi.Build), err
}
