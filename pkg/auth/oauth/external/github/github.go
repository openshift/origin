package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/external"
	"github.com/openshift/origin/pkg/util/http/links"
)

const (
	githubAuthorizeURL = "https://github.com/login/oauth/authorize"
	githubTokenURL     = "https://github.com/login/oauth/access_token"
	githubUserApiURL   = "https://api.github.com/user"
	githubUserOrgURL   = "https://api.github.com/user/orgs"
	githubOAuthScope   = "user:email"
	githubOrgScope     = "read:org"

	// https://developer.github.com/v3/#current-version
	// https://developer.github.com/v3/media/#request-specific-version
	githubAccept = "application/vnd.github.v3+json"
)

type provider struct {
	providerName         string
	clientID             string
	clientSecret         string
	allowedOrganizations sets.String
}

// https://developer.github.com/v3/users/#response
type githubUser struct {
	ID    uint64
	Login string
	Email string
	Name  string
}

// https://developer.github.com/v3/orgs/#response
type githubOrg struct {
	ID    uint64
	Login string
}

func NewProvider(providerName, clientID, clientSecret string, organizations []string) external.Provider {
	allowedOrganizations := sets.NewString()
	for _, org := range organizations {
		if len(org) > 0 {
			allowedOrganizations.Insert(strings.ToLower(org))
		}
	}

	return &provider{
		providerName:         providerName,
		clientID:             clientID,
		clientSecret:         clientSecret,
		allowedOrganizations: allowedOrganizations,
	}
}

func (p *provider) GetTransport() (http.RoundTripper, error) {
	return nil, nil
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p *provider) NewConfig() (*osincli.ClientConfig, error) {
	scopes := []string{githubOAuthScope}
	// if we're limiting to specific organizations, we also need to read their org membership
	if len(p.allowedOrganizations) > 0 {
		scopes = append(scopes, githubOrgScope)
	}

	config := &osincli.ClientConfig{
		ClientId:                 p.clientID,
		ClientSecret:             p.clientSecret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             githubAuthorizeURL,
		TokenUrl:                 githubTokenURL,
		Scope:                    strings.Join(scopes, " "),
	}
	return config, nil
}

// AddCustomParameters implements external/interfaces/Provider.AddCustomParameters
func (p provider) AddCustomParameters(req *osincli.AuthorizeRequest) {
}

// GetUserIdentity implements external/interfaces/Provider.GetUserIdentity
func (p *provider) GetUserIdentity(data *osincli.AccessData) (authapi.UserIdentityInfo, bool, error) {
	userdata := githubUser{}
	if _, err := getJSON(githubUserApiURL, data.AccessToken, &userdata); err != nil {
		return nil, false, err
	}
	if userdata.ID == 0 {
		return nil, false, errors.New("Could not retrieve GitHub id")
	}

	if len(p.allowedOrganizations) > 0 {
		userOrgs, err := getUserOrgs(data.AccessToken)
		if err != nil {
			return nil, false, err
		}

		if !userOrgs.HasAny(p.allowedOrganizations.List()...) {
			return nil, false, fmt.Errorf("User %s is not a member of any allowed organizations %v (user is a member of %v)", userdata.Login, p.allowedOrganizations.List(), userOrgs.List())
		}
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

// getUserOrgs retrieves the organization membership for the user with the given access token.
func getUserOrgs(token string) (sets.String, error) {
	// start with the empty set, and the initial org url
	userOrgs := sets.NewString()
	orgURL := githubUserOrgURL
	// track urls we've fetched to avoid cycles
	fetchedURLs := sets.NewString(orgURL)
	for {
		// fetch organizations
		organizations := []githubOrg{}
		links, err := getJSON(orgURL, token, &organizations)
		if err != nil {
			return nil, err
		}
		for _, org := range organizations {
			if len(org.Login) > 0 {
				userOrgs.Insert(strings.ToLower(org.Login))
			}
		}

		// see if we need to page
		// https://developer.github.com/v3/#link-header
		nextURL := links["next"]
		if len(nextURL) == 0 {
			// no next URL, we're done paging
			break
		}
		if fetchedURLs.Has(nextURL) {
			// break to avoid a loop
			break
		}
		// remember to avoid a loop
		fetchedURLs.Insert(nextURL)
		orgURL = nextURL
	}

	return userOrgs, nil
}

// getJSON fetches and deserializes JSON into the given object.
// returns a (possibly empty) map of link relations to url strings, or an error.
func getJSON(url string, token string, data interface{}) (map[string]string, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", fmt.Sprintf("bearer %s", token))
	req.Header.Set("Accept", githubAccept)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Non-200 response from GitHub API call %s: %d", url, res.StatusCode)
	}

	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		return nil, err
	}

	links := links.ParseLinks(res.Header.Get("Link"))
	return links, nil
}
