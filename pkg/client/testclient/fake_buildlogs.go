package testclient

import (
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	"github.com/openshift/origin/pkg/build/api"
)

// FakeBuildLogs implements BuildLogsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildLogs struct {
	Fake      *Fake
	Namespace string
}

// Get builds and returns a buildLog request
func (c *FakeBuildLogs) Get(name string, opt api.BuildLogOptions) *kclient.Request {
	c.Fake.Actions = append(c.Fake.Actions, ktestclient.FakeAction{Action: "proxy"})
	return &kclient.Request{}
}
