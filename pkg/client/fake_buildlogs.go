package client

import (
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
)

// FakeBuildLogs implements BuildLogInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeBuildLogs struct {
	Fake      *Fake
	Namespace string
}

// Redirect builds and returns a buildLog request
func (c *FakeBuildLogs) Redirect(name string) *kclient.Request {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "redirect"})
	return &kclient.Request{}
}
