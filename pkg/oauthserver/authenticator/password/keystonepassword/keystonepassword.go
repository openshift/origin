package keystonepassword

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"encoding/json"
	"strings"

	"github.com/golang/glog"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	tokens3 "github.com/rackspace/gophercloud/openstack/identity/v3/tokens"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"
	)

// keystonePasswordAuthenticator uses OpenStack keystone to authenticate a user by password
type keystonePasswordAuthenticator struct {
	providerName   string
	url            string
	client         *http.Client
	domainName     string
	identityMapper authapi.UserIdentityMapper
	user           userclient.UserInterface
}

// New creates a new password authenticator that uses OpenStack keystone to authenticate a user by password
// A custom transport can be provided (typically to customize TLS options like trusted roots or present a client certificate).
// If no transport is provided, http.DefaultTransport is used
func New(providerName string, url string, transport http.RoundTripper, domainName string, identityMapper authapi.UserIdentityMapper, user userclient.UserInterface) authenticator.Password {
	if transport == nil {
		transport = http.DefaultTransport
	}
	client := &http.Client{Transport: transport}
	return &keystonePasswordAuthenticator{providerName, url, client, domainName, identityMapper, user}
}

// Authenticate user and return his ID from Keystone
func GetUserIDv3(client *gophercloud.ProviderClient, options gophercloud.AuthOptions) (string, error) {
	// Override the generated service endpoint with the one returned by the version endpoint.
	v3Client := openstack.NewIdentityV3(client)

	// copy the auth options to a local variable that we can change. `options`
	// needs to stay as-is for reauth purposes
	v3Options := options

	var scope *tokens3.Scope
	if options.TenantID != "" {
		scope = &tokens3.Scope{
			ProjectID: options.TenantID,
		}
		v3Options.TenantID = ""
		v3Options.TenantName = ""
	} else {
		if options.TenantName != "" {
			scope = &tokens3.Scope{
				ProjectName: options.TenantName,
				DomainID:    options.DomainID,
				DomainName:  options.DomainName,
			}
			v3Options.TenantName = ""
		}
	}

	result := tokens3.Create(v3Client, v3Options, scope)

	token, err := result.ExtractToken()
	if err != nil {
		return "", err
	}

	catalog, err := result.ExtractServiceCatalog()
	if err != nil {
		return "", err
	}

	// Parse the body to obtain userID from the response
	out, err := json.Marshal(result.Body)
	if err != nil {
		return "", err
	}

	var bd map[string]interface{}

	err = json.Unmarshal(out, &bd)
	if err != nil {
		return "", err
	}

	client.TokenID = token.ID

	if options.AllowReauth {
		client.ReauthFunc = func() error {
			client.TokenID = ""
			return openstack.AuthenticateV3(client, options)
		}
	}
	client.EndpointLocator = func(opts gophercloud.EndpointOpts) (string, error) {
		return openstack.V3EndpointURL(catalog, opts)
	}

	return bd["token"].(map[string]interface{})["user"].(map[string]interface{})["id"].(string), nil
}

// AuthenticatePassword approves any login attempt which is successfully validated with Keystone
func (a keystonePasswordAuthenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	defer func() {
		if e := recover(); e != nil {
			utilruntime.HandleError(fmt.Errorf("Recovered panic: %v, %s", e, debug.Stack()))
		}
	}()

	// if password is missing, fail authentication immediately
	if len(password) == 0 {
		return nil, false, nil
	}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: a.url,
		Username:         username,
		Password:         password,
		DomainName:       a.domainName,
	}

	// Calling NewClient/Authenticate manually rather than simply calling AuthenticatedClient
	// in order to pass in a transport object that supports SSL
	client, err := openstack.NewClient(opts.IdentityEndpoint)
	if err != nil {
		glog.Warningf("Failed: Initializing openstack authentication client: %v", err)
		return nil, false, err
	}

	client.HTTPClient = *a.client
	userid, err := GetUserIDv3(client, opts)

	if err != nil {
		if responseErr, ok := err.(*gophercloud.UnexpectedResponseCodeError); ok {
			if responseErr.Actual == 401 {
				return nil, false, nil
			}
		}
		glog.Warningf("Failed: Calling openstack AuthenticateV3: %v", err)
		return nil, false, err
	}

	u, err := a.user.Get(username, metav1.GetOptions{})

	if err == nil {
		for _, idnt := range u.Identities {
			// identity has a format "my_keystone_provider:my_user_name"
			x := strings.SplitN(idnt, ":", 2)
			if x[0] == a.providerName {
				if x[1] != userid {
					// If the identifier of the new user does not match what is stored in the identity,
					// then the user's request to login to the system is rejected
					glog.Errorf("It is impossible to authenticate user \"%v\", because a user with the same " +
						"name is known by a different id. To resolve this issue, contact the administrator", username)
					return nil, false, nil
				}
				break
			}
		}
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, userid)
	identity.Extra[authapi.IdentityPreferredUsernameKey] = username

	user, err := a.identityMapper.UserFor(identity)
	if err != nil {
		glog.V(4).Infof("Error creating or updating mapping for: %#v due to %v", identity, err)
		return nil, false, err
	}
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)

	return user, true, nil
}
