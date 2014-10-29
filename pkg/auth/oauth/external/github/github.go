package github

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/RangelReale/osincli"
	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/external"
)

const (
	githubAuthorizeUrl = "https://github.com/login/oauth/authorize"
	githubTokenUrl     = "https://github.com/login/oauth/access_token"
	githubUserApiUrl   = "https://api.github.com/user"
	githubOauthScope   = "user:email"
)

type provider struct {
	client_id, client_secret string
}

type githubUser struct {
	ID    uint64
	Login string
	Email string
	Name  string
}

func NewProvider(client_id, client_secret string) external.Provider {
	return provider{client_id, client_secret}
}

func (p provider) NewConfig() (*osincli.ClientConfig, error) {
	config := &osincli.ClientConfig{
		ClientId:                 p.client_id,
		ClientSecret:             p.client_secret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             githubAuthorizeUrl,
		TokenUrl:                 githubTokenUrl,
		Scope:                    githubOauthScope,
	}
	return config, nil
}

func (p provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
}

func (p provider) GetUserInfo(data *osincli.AccessData) (api.UserInfo, bool, error) {

	req, _ := http.NewRequest("GET", githubUserApiUrl, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", data.AccessToken))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, false, err
	}

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
		return nil, false, fmt.Errorf("Could not retrieve GitHub id")
	}

	user := &api.DefaultUserInfo{
		Name: fmt.Sprintf("%d", userdata.ID),
		Extra: map[string]string{
			"name":  userdata.Name,
			"login": userdata.Login,
			"email": userdata.Email,
		},
	}
	return user, true, nil
}
