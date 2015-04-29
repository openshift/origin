package client

import ()

type OAuthAccessTokensInterface interface {
	OAuthAccessTokens() OAuthAccessTokenInterface
}

type OAuthAccessTokenInterface interface {
	Delete(name string) error
}

type oauthAccessTokenInterface struct {
	r *Client
}

func newOAuthAccessTokens(c *Client) *oauthAccessTokenInterface {
	return &oauthAccessTokenInterface{
		r: c,
	}
}

// Delete removes the OAuthAccessToken on server
func (c *oauthAccessTokenInterface) Delete(name string) (err error) {
	err = c.r.Delete().Resource("oAuthAccessTokens").Name(name).Do().Error()
	return
}
