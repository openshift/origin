package openid

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/sets"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/external"
)

const (
	// Standard claims (http://openid.net/specs/openid-connect-core-1_0.html#StandardClaims)
	SubjectClaim           = "sub"
	PreferredUsernameClaim = "preferred_username"
	EmailClaim             = "email"
	NameClaim              = "name"
)

type TokenValidator func(map[string]interface{}) error

type Config struct {
	ClientID     string
	ClientSecret string

	Scopes []string

	ExtraAuthorizeParameters map[string]string

	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string

	IDClaims                []string
	PreferredUsernameClaims []string
	EmailClaims             []string
	NameClaims              []string

	IDTokenValidator TokenValidator
}

type provider struct {
	providerName string
	transport    http.RoundTripper
	Config
}

// NewProvider returns an implementation of an OpenID Connect Authorization Code Flow
// See http://openid.net/specs/openid-connect-core-1_0.html#CodeFlowAuth
// ID Token decryption is not supported
// UserInfo decryption is not supported
func NewProvider(providerName string, transport http.RoundTripper, config Config) (external.Provider, error) {
	// TODO: Add support for discovery documents
	// see http://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig
	// e.g. https://accounts.google.com/.well-known/openid-configuration

	// Validate client id/secret
	if len(config.ClientID) == 0 {
		return nil, errors.New("ClientID is required")
	}
	if len(config.ClientSecret) == 0 {
		return nil, errors.New("ClientSecret is required")
	}

	// Validate url presence
	if len(config.AuthorizeURL) == 0 {
		return nil, errors.New("Authorize URL is required")
	} else if u, err := url.Parse(config.AuthorizeURL); err != nil {
		return nil, errors.New("Authorize URL is invalid")
	} else if u.Scheme != "https" {
		return nil, errors.New("Authorize URL must use https scheme")
	}

	if len(config.TokenURL) == 0 {
		return nil, errors.New("Token URL is required")
	} else if u, err := url.Parse(config.TokenURL); err != nil {
		return nil, errors.New("Token URL is invalid")
	} else if u.Scheme != "https" {
		return nil, errors.New("Token URL must use https scheme")
	}

	if len(config.UserInfoURL) > 0 {
		if u, err := url.Parse(config.UserInfoURL); err != nil {
			return nil, errors.New("UserInfo URL is invalid")
		} else if u.Scheme != "https" {
			return nil, errors.New("UserInfo URL must use https scheme")
		}
	}

	if !sets.NewString(config.Scopes...).Has("openid") {
		return nil, errors.New("Scopes must include openid")
	}

	if len(config.IDClaims) == 0 {
		return nil, errors.New("IDClaims must specify at least one claim")
	}

	return provider{providerName, transport, config}, nil
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p provider) NewConfig() (*osincli.ClientConfig, error) {
	config := &osincli.ClientConfig{
		ClientId:                 p.ClientID,
		ClientSecret:             p.ClientSecret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             p.AuthorizeURL,
		TokenUrl:                 p.TokenURL,
		Scope:                    strings.Join(p.Scopes, " "),
	}
	return config, nil
}

func (p provider) GetTransport() (http.RoundTripper, error) {
	return p.transport, nil
}

// AddCustomParameters implements external/interfaces/Provider.AddCustomParameters
func (p provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
	for k, v := range p.ExtraAuthorizeParameters {
		req.CustomParameters[k] = v
	}
}

// GetUserIdentity implements external/interfaces/Provider.GetUserIdentity
func (p provider) GetUserIdentity(data *osincli.AccessData) (authapi.UserIdentityInfo, bool, error) {
	// Token response MUST include id_token
	// http://openid.net/specs/openid-connect-core-1_0.html#TokenResponse
	idToken, ok := data.ResponseData["id_token"].(string)
	if !ok {
		return nil, false, fmt.Errorf("No id_token returned in %v", data.ResponseData)
	}

	// id_token MUST be a valid JWT
	idTokenClaims, err := decodeJWT(idToken)
	if err != nil {
		return nil, false, err
	}

	if p.IDTokenValidator != nil {
		if err := p.IDTokenValidator(idTokenClaims); err != nil {
			return nil, false, err
		}
	}

	// TODO: validate JWT
	// http://openid.net/specs/openid-connect-core-1_0.html#IDTokenValidation

	// id_token MUST contain a sub claim as the subject identifier
	// http://openid.net/specs/openid-connect-core-1_0.html#IDToken
	idTokenSubject, ok := idTokenClaims[SubjectClaim].(string)
	if !ok {
		return nil, false, fmt.Errorf("id_token did not contain a 'sub' claim: %#v", idTokenClaims)
	}

	// Use id_token claims by default
	claims := idTokenClaims

	// If we have a userinfo URL, use it to get more detailed claims
	if len(p.UserInfoURL) != 0 {
		userInfoClaims, err := fetchUserInfo(p.UserInfoURL, data.AccessToken, p.transport)
		if err != nil {
			return nil, false, err
		}

		// The sub (subject) Claim MUST always be returned in the UserInfo Response.
		// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
		userInfoSubject, ok := userInfoClaims[SubjectClaim].(string)
		if !ok {
			return nil, false, fmt.Errorf("userinfo response did not contain a 'sub' claim: %#v", userInfoClaims)
		}

		// The sub Claim in the UserInfo Response MUST be verified to exactly match the sub Claim in the ID Token;
		// if they do not match, the UserInfo Response values MUST NOT be used.
		// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
		if userInfoSubject != idTokenSubject {
			return nil, false, fmt.Errorf("userinfo 'sub' claim (%s) did not match id_token 'sub' claim (%s)", userInfoSubject, idTokenSubject)
		}

		// Merge in userinfo claims in case id_token claims contained some that userinfo did not
		for k, v := range userInfoClaims {
			claims[k] = v
		}
	}

	id, _ := getClaimValue(claims, p.IDClaims)
	if id == "" {
		return nil, false, fmt.Errorf("Could not retrieve id claim for %#v", p.IDClaims)
	}
	identity := authapi.NewDefaultUserIdentityInfo(p.providerName, id)

	if preferredUsername, _ := getClaimValue(claims, p.PreferredUsernameClaims); len(preferredUsername) != 0 {
		identity.Extra[authapi.IdentityPreferredUsernameKey] = preferredUsername
	}

	if email, _ := getClaimValue(claims, p.EmailClaims); len(email) != 0 {
		identity.Extra[authapi.IdentityEmailKey] = email
	}

	if name, _ := getClaimValue(claims, p.NameClaims); len(name) != 0 {
		identity.Extra[authapi.IdentityDisplayNameKey] = name
	}

	glog.V(4).Infof("identity=%v", identity)

	return identity, true, nil
}

func getClaimValue(data map[string]interface{}, claims []string) (string, error) {
	for _, claim := range claims {
		value, ok := data[claim]
		if !ok {
			continue
		}
		stringValue, ok := value.(string)
		if !ok {
			return "", fmt.Errorf("Claim %s was not a string type", claim)
		}
		if len(stringValue) > 0 {
			return stringValue, nil
		}
	}
	return "", errors.New("No value found")
}

// fetch and decode JSON from the given UserInfo URL
func fetchUserInfo(url, accessToken string, transport http.RoundTripper) (map[string]interface{}, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Non-200 response from UserInfo: %d, WWW-Authenticate=%s", resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	// The UserInfo Claims MUST be returned as the members of a JSON object
	// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	decoded := map[string]interface{}{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}

	return decoded, nil
}

// Decode JWT
// http://openid.net/specs/draft-jones-json-web-token-07.html
func decodeJWT(jwt string) (map[string]interface{}, error) {
	jwtParts := strings.Split(jwt, ".")
	if len(jwtParts) != 3 {
		return nil, fmt.Errorf("Invalid JSON Web Token: expected 3 parts, got %d", len(jwtParts))
	}

	// Re-pad, if needed
	encodedPayload := jwtParts[1]
	if l := len(encodedPayload) % 4; l != 0 {
		encodedPayload += strings.Repeat("=", 4-l)
	}

	// Decode base-64
	decodedPayload, err := base64.StdEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("Error decoding payload: %v", err)
	}

	// Parse JSON
	var data map[string]interface{}
	err = json.Unmarshal([]byte(decodedPayload), &data)
	if err != nil {
		return nil, fmt.Errorf("Error parsing token: %v", err)
	}

	return data, nil
}
