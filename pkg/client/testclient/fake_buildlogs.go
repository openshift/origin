package testclient

import (
	kclient "k8s.io/kubernetes/pkg/client"

	ktestclient "k8s.io/kubernetes/pkg/client/testclient"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// FakeBuildLogs implements BuildLogsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildLogs struct {
	Fake      *Fake
	Namespace string
}

func (c *FakeBuildLogs) Get(name string, opt buildapi.BuildLogOptions) *kclient.Request {
	action := ktestclient.GenericActionImpl{}
	action.Verb = "get"
	action.Namespace = c.Namespace
	action.Resource = "builds"
	action.Subresource = "logs"
	action.Value = opt

	_, _ = c.Fake.Invokes(action, &buildapi.BuildConfig{})
	return &kclient.Request{}
}
