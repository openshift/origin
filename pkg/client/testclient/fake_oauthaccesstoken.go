package testclient

import (
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
)

// FakeOAuthAccessTokens implements OAuthAccessTokenInterface. Meant to be embedded into a struct to get a default
// implementation. This makes faking out just the methods you want to test easier.
type FakeOAuthAccessTokens struct {
	Fake *Fake
}

// Delete mocks deleting an OAuthAccessToken
func (c *FakeOAuthAccessTokens) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, ktestclient.FakeAction{Action: "delete-oauthaccesstoken", Value: name})
	return nil
}
