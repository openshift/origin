package gitlab

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/oauth/external"
)

const (
	// Uses the GitLab User-API (http://doc.gitlab.com/ce/api/users.html#current-user)
	// and OAuth-Provider (http://doc.gitlab.com/ce/integration/oauth_provider.html)
	// with default OAuth scope (http://doc.gitlab.com/ce/api/users.html#current-user)
	// Requires GitLab 7.7.0 or higher (released prior to 2015-09-22, we support it in OS v3.1+)
	gitlabAuthorizePath = "/oauth/authorize"
	gitlabTokenPath     = "/oauth/token"
	gitlabOAuthScope    = "api"

	// the only thing different about the APIs is the endpoint
	// the JSON data is the same
	// v3 was removed in GitLab 11 2018-06-22 (deprecated in GitLab 9.5 2017-08-22)
	gitlabUserAPIPathV3 = "/api/v3/user"
	// v4 was added in GitLab 9 2017-03-22
	gitlabUserAPIPathV4 = "/api/v4/user"
)

type provider struct {
	providerName string
	transport    http.RoundTripper
	authorizeURL string
	tokenURL     string
	userAPIURLV3 string
	userAPIURLV4 string
	clientID     string
	clientSecret string
}

type gitlabUser struct {
	ID       uint64
	Username string
	Email    string
	Name     string
	Error    string
}

func NewProvider(providerName string, transport http.RoundTripper, URL, clientID, clientSecret string) (external.Provider, error) {
	// Create service URLs
	u, err := url.Parse(URL)
	if err != nil {
		return nil, errors.New("Host URL is invalid")
	}

	return &provider{
		providerName: providerName,
		transport:    transport,
		authorizeURL: appendPath(*u, gitlabAuthorizePath),
		tokenURL:     appendPath(*u, gitlabTokenPath),
		userAPIURLV3: appendPath(*u, gitlabUserAPIPathV3),
		userAPIURLV4: appendPath(*u, gitlabUserAPIPathV4),
		clientID:     clientID,
		clientSecret: clientSecret,
	}, nil
}

func appendPath(u url.URL, subpath string) string {
	u.Path = path.Join(u.Path, subpath)
	return u.String()
}

func (p *provider) GetTransport() (http.RoundTripper, error) {
	return p.transport, nil
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p *provider) NewConfig() (*osincli.ClientConfig, error) {
	config := &osincli.ClientConfig{
		ClientId:                 p.clientID,
		ClientSecret:             p.clientSecret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             p.authorizeURL,
		TokenUrl:                 p.tokenURL,
		Scope:                    gitlabOAuthScope,
	}
	return config, nil
}

// AddCustomParameters implements external/interfaces/Provider.AddCustomParameters
func (p *provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
}

// GetUserIdentity implements external/interfaces/Provider.GetUserIdentity
func (p *provider) GetUserIdentity(data *osincli.AccessData) (authapi.UserIdentityInfo, bool, error) {
	userdata, err := p.getUserData(data.AccessToken)
	if err != nil {
		return nil, false, err
	}

	identity := authapi.NewDefaultUserIdentityInfo(p.providerName, fmt.Sprintf("%d", userdata.ID))
	if len(userdata.Name) > 0 {
		identity.Extra[authapi.IdentityDisplayNameKey] = userdata.Name
	}
	if len(userdata.Username) > 0 {
		identity.Extra[authapi.IdentityPreferredUsernameKey] = userdata.Username
	}
	if len(userdata.Email) > 0 {
		identity.Extra[authapi.IdentityEmailKey] = userdata.Email
	}
	glog.V(4).Infof("Got identity=%#v", identity)

	return identity, true, nil
}

func (p *provider) getUserData(token string) (*gitlabUser, error) {
	// try the v4 API first
	userdata, err := p.getUserDataURL(token, p.userAPIURLV4)
	if err == nil {
		return userdata, nil
	}
	// fallback to the v3 API
	return p.getUserDataURL(token, p.userAPIURLV3)
}

func (p *provider) getUserDataURL(token, url string) (*gitlabUser, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", token))

	client := &http.Client{Transport: p.transport}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	userdata := &gitlabUser{}
	if err := json.Unmarshal(body, userdata); err != nil {
		return nil, err
	}

	if len(userdata.Error) > 0 {
		return nil, errors.New(userdata.Error)
	}

	if userdata.ID == 0 {
		return nil, errors.New("could not retrieve GitLab id")
	}

	return userdata, nil
}
