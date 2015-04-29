package client

import ()

type FakeOAuthAccessTokens struct {
	Fake *Fake
}

func (c *FakeOAuthAccessTokens) Delete(name string) error {
	c.Fake.Actions = append(c.Fake.Actions, FakeAction{Action: "delete-oauthaccesstoken", Value: name})
	return nil
}
