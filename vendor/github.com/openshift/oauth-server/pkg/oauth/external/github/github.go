package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/RangelReale/osincli"
	"k8s.io/klog"

	authapi "github.com/openshift/oauth-server/pkg/api"
	"github.com/openshift/oauth-server/pkg/oauth/external"
	"github.com/openshift/oauth-server/pkg/oauth/external/github/links"
)

const (
	// GitHub OAuth endpoints
	defaultGithubAuthorizeURL = "https://github.com/login/oauth/authorize"
	defaultGithubTokenURL     = "https://github.com/login/oauth/access_token"

	// GitHub User API Endpoints
	defaultGithubUserApiURL   = "https://api.github.com/user"
	defaultGithubUserOrgURL   = "https://api.github.com/user/orgs"
	defaultGithubUserTeamURL  = "https://api.github.com/user/teams"
	defaultGithubUserEmailURL = "https://api.github.com/user/emails"

	// GitHub Enterprise OAuth endpoints with hostname substitution
	enterpriseGithubAuthorizeURL = "https://%s/login/oauth/authorize"
	enterpriseGithubTokenURL     = "https://%s/login/oauth/access_token"

	// GitHub User API endpoints with hostname substitution
	enterpriseGithubUserApiURL   = "https://%s/api/v3/user"
	enterpriseGithubUserOrgURL   = "https://%s/api/v3/user/orgs"
	enterpriseGithubUserTeamURL  = "https://%s/api/v3/user/teams"
	enterpriseGithubUserEmailURL = "https://%s/api/v3/user/emails"

	// GitHub OAuth scopes, see provider.NewConfig
	githubOAuthScope = "user:email"
	githubOrgScope   = "read:org"

	// https://developer.github.com/v3/#current-version
	// https://developer.github.com/v3/media/#request-specific-version
	githubAccept = "application/vnd.github.v3+json"
)

type provider struct {
	providerName         string
	clientID             string
	clientSecret         string
	allowedOrganizations sets.String
	allowedTeams         sets.String

	// OAuth endpoints
	githubAuthorizeURL string
	githubTokenURL     string

	// User API endpoints
	githubUserApiURL   string
	githubUserOrgURL   string
	githubUserTeamURL  string
	githubUserEmailURL string

	// incorporates the CA bundle which may be required when GitHub Enterprise is used
	transport http.RoundTripper
}

// https://developer.github.com/v3/users/#response
type githubUser struct {
	ID    uint64
	Login string
	Email string
	Name  string
}

// https://developer.github.com/v3/users/emails/#response
type githubUserEmail struct {
	Email   string
	Primary bool
}

// https://developer.github.com/v3/orgs/#response
type githubOrg struct {
	ID    uint64
	Login string
}

// https://developer.github.com/v3/orgs/teams/#response-12
type githubTeam struct {
	ID           uint64
	Slug         string
	Organization githubOrg
}

var _ external.Provider = &provider{}

func NewProvider(providerName, clientID, clientSecret, hostname string, transport http.RoundTripper, organizations, teams []string) external.Provider {
	allowedOrganizations := sets.NewString()
	for _, org := range organizations {
		if len(org) > 0 {
			allowedOrganizations.Insert(strings.ToLower(org))
		}
	}

	allowedTeams := sets.NewString()
	for _, team := range teams {
		if len(team) > 0 {
			allowedTeams.Insert(strings.ToLower(team))
		}
	}

	p := &provider{
		providerName:         providerName,
		clientID:             clientID,
		clientSecret:         clientSecret,
		allowedOrganizations: allowedOrganizations,
		allowedTeams:         allowedTeams,
		transport:            transport,
	}

	if len(hostname) != 0 {
		p.githubAuthorizeURL = fmt.Sprintf(enterpriseGithubAuthorizeURL, hostname)
		p.githubTokenURL = fmt.Sprintf(enterpriseGithubTokenURL, hostname)
		p.githubUserApiURL = fmt.Sprintf(enterpriseGithubUserApiURL, hostname)
		p.githubUserOrgURL = fmt.Sprintf(enterpriseGithubUserOrgURL, hostname)
		p.githubUserTeamURL = fmt.Sprintf(enterpriseGithubUserTeamURL, hostname)
		p.githubUserEmailURL = fmt.Sprintf(enterpriseGithubUserEmailURL, hostname)
	} else {
		p.githubAuthorizeURL = defaultGithubAuthorizeURL
		p.githubTokenURL = defaultGithubTokenURL
		p.githubUserApiURL = defaultGithubUserApiURL
		p.githubUserOrgURL = defaultGithubUserOrgURL
		p.githubUserTeamURL = defaultGithubUserTeamURL
		p.githubUserEmailURL = defaultGithubUserEmailURL
	}

	return p
}

func (p *provider) GetTransport() (http.RoundTripper, error) {
	return p.transport, nil
}

// NewConfig implements external/interfaces/Provider.NewConfig
func (p *provider) NewConfig() (*osincli.ClientConfig, error) {
	scopes := []string{githubOAuthScope}
	// if we're limiting to specific organizations or teams, we also need to read their org membership
	if len(p.allowedOrganizations) > 0 || len(p.allowedTeams) > 0 {
		scopes = append(scopes, githubOrgScope)
	}

	config := &osincli.ClientConfig{
		ClientId:                 p.clientID,
		ClientSecret:             p.clientSecret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             p.githubAuthorizeURL,
		TokenUrl:                 p.githubTokenURL,
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
	if _, err := p.getJSON(p.githubUserApiURL, data.AccessToken, &userdata); err != nil {
		return nil, false, err
	}
	if userdata.ID == 0 {
		return nil, false, errors.New("Could not retrieve GitHub id")
	}

	if len(p.allowedOrganizations) > 0 {
		userOrgs, err := p.getUserOrgs(data.AccessToken)
		if err != nil {
			return nil, false, err
		}

		if !userOrgs.HasAny(p.allowedOrganizations.List()...) {
			return nil, false, fmt.Errorf("User %s is not a member of any allowed organizations %v (user is a member of %v)", userdata.Login, p.allowedOrganizations.List(), userOrgs.List())
		}
		klog.V(4).Infof("User %s is a member of organizations %v)", userdata.Login, userOrgs.List())
	}
	if len(p.allowedTeams) > 0 {
		userTeams, err := p.getUserTeams(data.AccessToken)
		if err != nil {
			return nil, false, err
		}

		if !userTeams.HasAny(p.allowedTeams.List()...) {
			return nil, false, fmt.Errorf("User %s is not a member of any allowed teams %v (user is a member of %v)", userdata.Login, p.allowedTeams.List(), userTeams.List())
		}
		klog.V(4).Infof("User %s is a member of teams %v)", userdata.Login, userTeams.List())
	}

	// The returned email is empty if the user has not specified a public email address in their profile
	if len(userdata.Email) == 0 {
		email, err := p.getUserEmail(data.AccessToken)
		if err == nil {
			userdata.Email = email
		} else {
			klog.V(4).Infof("Failed to get user email information %#v", err)
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
	klog.V(4).Infof("Got identity=%#v", identity)

	return identity, true, nil
}

// getUserOrgs retrieves the organization membership for the user with the given access token.
func (p *provider) getUserOrgs(token string) (sets.String, error) {
	userOrgs := sets.NewString()
	err := p.page(p.githubUserOrgURL, token,
		func() interface{} {
			return &[]githubOrg{}
		},
		func(obj interface{}) error {
			for _, org := range *(obj.(*[]githubOrg)) {
				if len(org.Login) > 0 {
					userOrgs.Insert(strings.ToLower(org.Login))
				}
			}
			return nil
		},
	)
	return userOrgs, err
}

// getUserTeams retrieves the team memberships for the user with the given access token.
func (p *provider) getUserTeams(token string) (sets.String, error) {
	userTeams := sets.NewString()
	err := p.page(p.githubUserTeamURL, token,
		func() interface{} {
			return &[]githubTeam{}
		},
		func(obj interface{}) error {
			for _, team := range *(obj.(*[]githubTeam)) {
				if len(team.Slug) > 0 && len(team.Organization.Login) > 0 {
					userTeams.Insert(strings.ToLower(team.Organization.Login + "/" + team.Slug))
				}
			}
			return nil
		},
	)
	return userTeams, err
}

var errStopEmail = errors.New("done iterating over email because we found primary")

// getUserEmail retrieves the primary email for the user with the given access token.
func (p *provider) getUserEmail(token string) (string, error) {
	var email string
	err := p.page(p.githubUserEmailURL, token,
		func() interface{} {
			return &[]githubUserEmail{}
		},
		func(obj interface{}) error {
			for _, userEmail := range *(obj.(*[]githubUserEmail)) {
				// store the email regardless of if it the primary in case we somehow never get a primary one
				email = userEmail.Email
				if userEmail.Primary {
					return errStopEmail
				}
			}
			return nil
		},
	)
	// this error just stops iteration early on the first primary email (there should only ever be one primary)
	if err == errStopEmail {
		return email, nil
	}
	return email, err
}

// page fetches the intialURL, and follows "next" links
func (p *provider) page(initialURL, token string, newObj func() interface{}, processObj func(interface{}) error) error {
	// track urls we've fetched to avoid cycles
	url := initialURL
	fetchedURLs := sets.NewString(url)
	for {
		// fetch and process
		obj := newObj()
		links, err := p.getJSON(url, token, obj)
		if err != nil {
			return err
		}
		if err := processObj(obj); err != nil {
			return err
		}

		// see if we need to page
		// https://developer.github.com/v3/#link-header
		url = links["next"]
		if len(url) == 0 {
			// no next URL, we're done paging
			break
		}
		if fetchedURLs.Has(url) {
			// break to avoid a loop
			break
		}
		// remember to avoid a loop
		fetchedURLs.Insert(url)
	}
	return nil
}

// getJSON fetches and deserializes JSON into the given object.
// returns a (possibly empty) map of link relations to url strings, or an error.
func (p *provider) getJSON(url string, token string, data interface{}) (map[string]string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", githubAccept)

	res, err := p.transport.RoundTrip(req)
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

	return links.ParseLinks(res.Header.Get("Link")), nil
}
