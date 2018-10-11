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

	"k8s.io/apimachinery/pkg/util/sets"

	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/oauth/external"
)

const (
	// Standard claims (http://openid.net/specs/openid-connect-core-1_0.html#StandardClaims)
	subjectClaim = "sub"

	// The claim containing a map of endpoint references per claim.
	// OIDC Connect Core 1.0, section 5.6.2.
	claimNamesKey = "_claim_names"
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
	GroupsClaims            []string

	IDTokenValidator TokenValidator
}

type provider struct {
	providerName string
	transport    http.RoundTripper
	allClaims    sets.String
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
		return nil, errors.New("authorize URL is required")
	} else if u, err := url.Parse(config.AuthorizeURL); err != nil {
		return nil, errors.New("authorize URL is invalid")
	} else if u.Scheme != "https" {
		return nil, errors.New("authorize URL must use https scheme")
	}

	if len(config.TokenURL) == 0 {
		return nil, errors.New("token URL is required")
	} else if u, err := url.Parse(config.TokenURL); err != nil {
		return nil, errors.New("token URL is invalid")
	} else if u.Scheme != "https" {
		return nil, errors.New("token URL must use https scheme")
	}

	if len(config.UserInfoURL) > 0 {
		if u, err := url.Parse(config.UserInfoURL); err != nil {
			return nil, errors.New("UserInfo URL is invalid")
		} else if u.Scheme != "https" {
			return nil, errors.New("UserInfo URL must use https scheme")
		}
	}

	if !sets.NewString(config.Scopes...).Has("openid") {
		return nil, errors.New("scopes must include openid")
	}

	if len(config.IDClaims) == 0 {
		return nil, errors.New("IDClaims must specify at least one claim")
	}

	// we purposefully do not store IDClaims here because that could lead to a different
	// identity object name based on a distributed claim that was previously ignored.
	// thus we cannot support fields that use distributed claims as IDClaims.
	// this is fine because only the sub claim should be used as an ID claim.
	// the sub claim is both required and must be stable per the OIDC spec.
	allClaims := sets.NewString()
	allClaims.Insert(config.PreferredUsernameClaims...)
	allClaims.Insert(config.EmailClaims...)
	allClaims.Insert(config.NameClaims...)
	allClaims.Insert(config.GroupsClaims...)

	return &provider{providerName: providerName, transport: transport, allClaims: allClaims, Config: config}, nil
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p *provider) NewConfig() (*osincli.ClientConfig, error) {
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

func (p *provider) GetTransport() (http.RoundTripper, error) {
	return p.transport, nil
}

// AddCustomParameters implements external/interfaces/Provider.AddCustomParameters
func (p *provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
	for k, v := range p.ExtraAuthorizeParameters {
		req.CustomParameters[k] = v
	}
}

// GetUserIdentity implements external/interfaces/Provider.GetUserIdentity
func (p *provider) GetUserIdentity(data *osincli.AccessData) (authapi.UserIdentityInfo, bool, error) {
	// Token response MUST include id_token
	// http://openid.net/specs/openid-connect-core-1_0.html#TokenResponse
	idToken, ok := getClaimValue(data.ResponseData, "id_token")
	if !ok {
		return nil, false, fmt.Errorf("no id_token returned in %#v", data.ResponseData)
	}

	// id_token MUST be a valid JWT
	idTokenClaims, err := p.decodeJWT(idToken, false)
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
	idTokenSubject, ok := getClaimValue(idTokenClaims, subjectClaim)
	if !ok {
		return nil, false, fmt.Errorf("id_token did not contain a 'sub' claim: %#v", idTokenClaims)
	}

	// Use id_token claims by default
	claims := idTokenClaims

	// If we have a userinfo URL, use it to get more detailed claims
	if len(p.UserInfoURL) != 0 {
		userInfoClaims, err := p.fetchUserInfo(p.UserInfoURL, data.AccessToken)
		if err != nil {
			return nil, false, err
		}

		// The sub (subject) Claim MUST always be returned in the UserInfo Response.
		// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
		userInfoSubject, ok := getClaimValue(userInfoClaims, subjectClaim)
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

	glog.V(5).Infof("openid claims: %#v", claims)

	id, ok := getClaimValue(claims, p.IDClaims...)
	if !ok {
		return nil, false, fmt.Errorf("could not retrieve id claim for %#v from %#v", p.IDClaims, claims)
	}

	identity := authapi.NewDefaultUserIdentityInfo(p.providerName, id)

	if preferredUsername, ok := getClaimValue(claims, p.PreferredUsernameClaims...); ok {
		identity.Extra[authapi.IdentityPreferredUsernameKey] = preferredUsername
	}

	if email, ok := getClaimValue(claims, p.EmailClaims...); ok {
		identity.Extra[authapi.IdentityEmailKey] = email
	}

	if name, ok := getClaimValue(claims, p.NameClaims...); ok {
		identity.Extra[authapi.IdentityDisplayNameKey] = name
	}

	identity.ProviderGroups = getClaimValues(claims, p.GroupsClaims...)

	glog.V(4).Infof("identity=%#v", identity)

	return identity, true, nil
}

func getClaimValue(data map[string]interface{}, claims ...string) (string, bool) {
	for _, claim := range claims {
		s, _ := data[claim].(string)
		if len(s) > 0 {
			return s, true
		}
	}
	return "", false
}

func getClaimValues(data map[string]interface{}, claims ...string) []string {
	var out []string
	for _, claim := range claims {
		switch t := data[claim].(type) {
		case string:
			out = appendIfNonEmpty(out, t)
		case []string:
			for _, s := range t {
				out = appendIfNonEmpty(out, s)
			}
		case []interface{}:
			for _, v := range t {
				s, _ := v.(string)
				out = appendIfNonEmpty(out, s)
			}
		default:
			continue
		}
	}
	return out
}

func appendIfNonEmpty(in []string, s string) []string {
	if len(s) > 0 {
		in = append(in, s)
	}
	return in
}

// fetch and decode JSON from the given UserInfo URL
func (p *provider) fetchUserInfo(url, accessToken string) (map[string]interface{}, error) {
	// The UserInfo Claims MUST be returned as the members of a JSON object
	// http://openid.net/specs/openid-connect-core-1_0.html#UserInfoResponse
	data, err := p.fetchURL(url, accessToken)
	if err != nil {
		return nil, err
	}
	return p.getClaims(data, false)
}

func (p *provider) fetchURL(url, accessToken string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if len(accessToken) != 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	}

	client := &http.Client{Transport: p.transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("non-200 response from %s: %d, WWW-Authenticate=%s", url, resp.StatusCode, resp.Header.Get("WWW-Authenticate"))
	}

	return ioutil.ReadAll(resp.Body)
}

// Decode JWT
// https://tools.ietf.org/html/rfc7519
func (p *provider) decodeJWT(jwt string, isEndpointJWT bool) (map[string]interface{}, error) {
	jwtParts := strings.Split(jwt, ".")
	if len(jwtParts) != 3 {
		return nil, fmt.Errorf("invalid JSON Web Token: expected 3 parts, got %d", len(jwtParts))
	}

	// Re-pad, if needed
	encodedPayload := jwtParts[1]
	if l := len(encodedPayload) % 4; l != 0 {
		encodedPayload += strings.Repeat("=", 4-l)
	}

	// Decode base64url
	decodedPayload, err := base64.URLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %v", err)
	}

	// Parse JSON
	return p.getClaims(decodedPayload, isEndpointJWT)
}

func (p *provider) getClaims(data []byte, isEndpointJWT bool) (map[string]interface{}, error) {
	// data can contain secret information such as access tokens so make sure not to log it or include it in errors

	var claims map[string]interface{}
	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, fmt.Errorf("error parsing standard claims: %v", err)
	}

	if _, ok := claims[claimNamesKey]; !ok || isEndpointJWT {
		// no distributed claims or recursive call from fetchEndpoint
		return claims, nil
	}

	// fetching distributed claims is not allowed to fail
	// enforce that behavior by wrapping the logic in a func that does not return anything
	// this is required so that we can work with broken OIDC implementations like Azure
	p.fetchDistributedClaims(data,
		func(srcClaims map[string]interface{}) {
			// merge new distributed claims
			for k, v := range srcClaims {
				claims[k] = v
			}
		})

	return claims, nil
}

func (p *provider) fetchDistributedClaims(data []byte, merge func(map[string]interface{})) {
	// decoding the same data twice is a bit wasteful, but distributed claims are rare and JWTs are small
	var distClaims distributedClaims
	if err := json.Unmarshal(data, &distClaims); err != nil {
		// fetching distributed claims is not allowed to fail
		// this should never happen since data should have already passed the initial json.Unmarshal
		glog.V(5).Infof("error parsing distributed claims: %v", err)
		return
	}

	completedSources := sets.NewString()
	for name, src := range distClaims.Names {
		// only make network calls for distributed claims we may care about
		// and only fetch each source once
		if p.allClaims.Has(name) && !completedSources.Has(src) {
			ep, ok := distClaims.Sources[src]
			if !ok {
				// malformed distributed claim
				glog.V(4).Infof("_claim_names %q contained a source %q not in _claims_sources", name, src)
				continue
			}
			if len(ep.URL) == 0 {
				// ignore possibly aggregated claim
				continue
			}
			srcClaims, err := p.fetchEndpoint(ep)
			if err != nil {
				glog.V(4).Infof("failed to fetch endpoint for claim %q with source %q: %v", name, src, err)
				continue
			}
			// merge new distributed claims
			merge(srcClaims)
			completedSources.Insert(src)
		}
	}
}

func (p *provider) fetchEndpoint(ep endpoint) (map[string]interface{}, error) {
	jwtBytes, err := p.fetchURL(ep.URL, ep.AccessToken)
	if err != nil {
		return nil, err
	}
	// passing true for isEndpointJWT means just get the claims out of the JWT
	// i.e. do not attempt to fetch any distributed claims or otherwise make network calls
	// this prevents infinite recursion via decodeJWT -> getClaims -> fetchEndpoint -> decodeJWT
	return p.decodeJWT(string(jwtBytes), true)
}

type distributedClaims struct {
	Names   map[string]string   `json:"_claim_names,omitempty"`
	Sources map[string]endpoint `json:"_claim_sources,omitempty"`
}

// endpoint represents an OIDC distributed claims endpoint.
type endpoint struct {
	// URL to use to request the distributed claim.  This URL is expected to be
	// prefixed by one of the known issuer URLs.
	URL string `json:"endpoint,omitempty"`
	// AccessToken is the bearer token to use for access.  If empty, it is
	// not used.  Access token is optional per the OIDC distributed claims
	// specification.
	// See: http://openid.net/specs/openid-connect-core-1_0.html#DistributedExample
	AccessToken string `json:"access_token,omitempty"`
}
