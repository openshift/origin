package testclient

import (
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/testing/core"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// FakeBuildLogs implements BuildLogsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildLogs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeBuildLogs) Get(name string, opt buildapi.BuildLogOptions) *restclient.Request {
	action := core.GenericActionImpl{}
	action.Verb = "get"
	action.Namespace = c.Namespace
	action.Resource = buildsResource
	action.Subresource = "log"
	action.Value = opt

	_, _ = c.Fake.Invokes(action, &buildapi.BuildConfig{})
	return &restclient.Request{}
}
