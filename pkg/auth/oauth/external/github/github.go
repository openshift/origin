package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/external"
)

const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserApiURL   = "https://api.github.com/user"
	githubOAuthScope   = "user:email"
)

type provider struct {
	providerName, clientID, clientSecret string
}

type githubUser struct {
	ID    uint64
	Login string
	Email string
	Name  string
}

func NewProvider(providerName, clientID, clientSecret string) external.Provider {
	return provider{providerName, clientID, clientSecret}
}

func (p provider) GetTransport() (http.RoundTripper, error) {
	return nil, nil
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p provider) NewConfig() (*osincli.ClientConfig, error) {
	config := &osincli.ClientConfig{
		ClientId:                 p.clientID,
		ClientSecret:             p.clientSecret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             githubAuthorizeURL,
		TokenUrl:                 githubTokenURL,
		Scope:                    githubOAuthScope,
	}
	return config, nil
}

// AddCustomParameters implements external/interfaces/Provider.AddCustomParameters
func (p provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
}

// GetUserIdentity implements external/interfaces/Provider.GetUserIdentity
func (p provider) GetUserIdentity(data *osincli.AccessData) (authapi.UserIdentityInfo, bool, error) {
	req, _ := http.NewRequest("GET", githubUserApiURL, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", data.AccessToken))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, false, err
	}

	userdata := githubUser{}
	err = json.Unmarshal(body, &userdata)
	if err != nil {
		return nil, false, err
	}

	if userdata.ID == 0 {
		return nil, false, errors.New("Could not retrieve GitHub id")
	}

	identity := authapi.NewDefaultUserIdentityInfo(p.providerName, fmt.Sprintf("%d", userdata.ID))
	if len(userdata.Name) > 0 {
		identity.Extra[authapi.IdentityDisplayNameKey] = userdata.Name
	}
	if len(userdata.Login) > 0 {
		identity.Extra[authapi.IdentityPreferredUsernameKey] = userdata.Login
	}
	if len(userdata.Email) > 0 {
		identity.Extra[authapi.IdentityEmailKey] = userdata.Email
	}
	glog.V(4).Infof("Got identity=%#v", identity)

	return identity, true, nil
}
