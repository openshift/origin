package client

import (
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
)

// FakeBuildLogs implements BuildLogsInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildLogs struct {
	Fake      *Fake
	Namespace string
}

// Get builds and returns a buildLog request
func (c *FakeBuildLogs) Get(name string) *kclient.Request {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "proxy"})
	return &kclient.Request{}
}
