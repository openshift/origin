package osincli

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
)

type AccessRequestType string

const (
	AUTHORIZATION_CODE AccessRequestType = "authorization_code"
	REFRESH_TOKEN                        = "refresh_token"
	PASSWORD                             = "password"
	CLIENT_CREDENTIALS                   = "client_credentials"
	IMPLICIT                             = "__implicit"
)

// Access request information
type AccessRequest struct {
	client           *Client
	Type             AccessRequestType
	AuthorizeData    *AuthorizeData
	CustomParameters map[string]string
}

// Access data
type AccessData struct {
	TokenType    string
	AccessToken  string
	RefreshToken string
	Expiration   *int32
	ResponseData ResponseData
}

func (c *Client) NewAccessRequest(t AccessRequestType, ad *AuthorizeData) *AccessRequest {
	return &AccessRequest{
		client:           c,
		Type:             t,
		AuthorizeData:    ad,
		CustomParameters: make(map[string]string),
	}
}

// Generate a token url. This may not be the exact url that is used,
// as it may be submited with basic authentication or form-encoded.
func (c *AccessRequest) GetTokenUrl() *url.URL {
	u := *c.client.configcache.tokenUrl
	uq := u.Query()
	uq.Add("grant_type", string(c.Type))

	switch c.Type {
	case PASSWORD:
		// https://tools.ietf.org/html/rfc6749#section-4.3.2
		// grant_type, username, password, scope (optional)

		// Avoid double-adding a username parameter if it was specified as a custom parameter and not in AuthorizeData
		// This ensures callers that previously used CustomParameters to pass username continue to work
		_, hasCustomUsername := c.CustomParameters["username"]
		if len(c.AuthorizeData.Username) > 0 || !hasCustomUsername {
			uq.Add("username", c.AuthorizeData.Username)
		}

		// Avoid double-adding a password parameter if it was specified as a custom parameter and not in AuthorizeData
		// This ensures callers that previously used CustomParameters to pass password continue to work
		_, hasCustomPassword := c.CustomParameters["password"]
		if len(c.AuthorizeData.Password) > 0 || !hasCustomPassword {
			uq.Add("password", c.AuthorizeData.Password)
		}

		// Avoid double-adding a scope parameter if it was specified as a customer parameter
		// This ensures callers that previously used CustomParameters to pass scope continue to work
		_, hasCustomScope := c.CustomParameters["scope"]
		if !hasCustomScope && c.client.config.Scope != "" {
			uq.Add("scope", c.client.config.Scope)
		}

	case REFRESH_TOKEN:
		// https://tools.ietf.org/html/rfc6749#section-6
		// grant_type, refresh_token, scope (optional, defaults to same as original access token)
		uq.Add("refresh_token", c.AuthorizeData.Code)

	case AUTHORIZATION_CODE:
		// https://tools.ietf.org/html/rfc6749#section-4.1.3
		// grant_type, code, redirect_uri
		uq.Add("code", c.AuthorizeData.Code)
		uq.Add("redirect_uri", c.client.config.RedirectUrl)
		if c.client.config.CodeVerifier != "" {
			uq.Add("code_verifier", c.client.config.CodeVerifier)
		}

	case CLIENT_CREDENTIALS:
		// https://tools.ietf.org/html/rfc6749#section-4.4.2
		// grant_type, scope

		// Avoid double-adding a scope parameter if it was specified as a customer parameter
		// This ensures callers that previously used CustomParameters to pass scope continue to work
		_, hasCustomScope := c.CustomParameters["scope"]
		if !hasCustomScope && c.client.config.Scope != "" {
			uq.Add("scope", c.client.config.Scope)
		}
	}

	if c.client.config.SendClientSecretInParams {
		uq.Add("client_id", c.client.config.ClientId)
		uq.Add("client_secret", c.client.config.ClientSecret)
	}
	if c.CustomParameters != nil {
		for pn, pv := range c.CustomParameters {
			uq.Add(pn, pv)
		}
	}
	u.RawQuery = uq.Encode()

	return &u
}

// Send a token request and capture data.
// On OAuth2 error, and osincli.Error is returned as error
func (c *AccessRequest) GetToken() (*AccessData, error) {
	// build token url
	tu := c.GetTokenUrl()
	var ba *BasicAuth
	if !c.client.config.SendClientSecretInParams {
		ba = &BasicAuth{Username: c.client.config.ClientId, Password: c.client.config.ClientSecret}
	}

	// return value
	ret := &AccessData{
		ResponseData: make(ResponseData),
	}

	// download data
	m := "POST"
	if c.client.config.UseGetAccessRequest {
		m = "GET"
	}
	err := downloadData(m, tu, ba, c.client.Transport, ret.ResponseData)
	if err != nil {
		return nil, err
	}

	// extract and convert received data
	token_type, ok := ret.ResponseData["token_type"]
	if !ok {
		return nil, errors.New("Invalid parameters received")
	}
	ret.TokenType = fmt.Sprintf("%v", token_type)

	access_token, ok := ret.ResponseData["access_token"]
	if !ok {
		return nil, errors.New("Invalid parameters received")
	}
	ret.AccessToken = fmt.Sprintf("%v", access_token)

	refresh_token, ok := ret.ResponseData["refresh_token"]
	if !ok {
		ret.RefreshToken = ""
	} else {
		ret.RefreshToken = fmt.Sprintf("%v", refresh_token)
	}

	expires_in_raw, ok := ret.ResponseData["expires_in"]
	if ok {
		rv := reflect.ValueOf(expires_in_raw)
		switch rv.Kind() {
		case reflect.Float64:
			// encoding/json always convert numbers fo float64
			ret.Expiration = new(int32)
			*ret.Expiration = int32(rv.Float())
		case reflect.String:
			// if string convert to integer
			ei, err := strconv.ParseInt(rv.String(), 10, 32)
			if err != nil {
				return nil, err
			}
			ret.Expiration = new(int32)
			*ret.Expiration = int32(ei)
		default:
			return nil, errors.New("Invalid parameter value")
		}
	}

	return ret, nil
}
