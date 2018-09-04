package keystonepassword

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/golang/glog"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	tokens3 "github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/user"

	authapi "github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/identitymapper"
)

// keystonePasswordAuthenticator uses OpenStack keystone to authenticate a user by password
type keystonePasswordAuthenticator struct {
	providerName        string
	url                 string
	client              *http.Client
	domainName          string
	identityMapper      authapi.UserIdentityMapper
	useKeystoneIdentity bool
}

// New creates a new password authenticator that uses OpenStack keystone to authenticate a user by password
// A custom transport can be provided (typically to customize TLS options like trusted roots or present a client certificate).
// If no transport is provided, http.DefaultTransport is used
func New(providerName string, url string, transport http.RoundTripper, domainName string, identityMapper authapi.UserIdentityMapper, useKeystoneIdentity bool) authenticator.Password {
	if transport == nil {
		transport = http.DefaultTransport
	}
	client := &http.Client{Transport: transport}
	return &keystonePasswordAuthenticator{providerName, url, client, domainName, identityMapper, useKeystoneIdentity}
}

// Authenticate user and return his Keystone ID
func getUserIDv3(client *gophercloud.ProviderClient, options tokens3.AuthOptionsBuilder, eo gophercloud.EndpointOpts) (string, error) {
	// Override the generated service endpoint with the one returned by the version endpoint.
	v3Client, err := openstack.NewIdentityV3(client, eo)
	if err != nil {
		return "", err
	}

	// Issue new unscoped token
	result := tokens3.Create(v3Client, options)
	if result.Err != nil {
		return "", result.Err
	}

	user, err := result.ExtractUser()
	if err != nil {
		return "", err
	}

	return user.ID, nil
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
	userid, err := getUserIDv3(client, &opts, gophercloud.EndpointOpts{})

	if err != nil {
		if _, ok := err.(gophercloud.ErrDefault401); ok {
			return nil, false, nil
		}
		glog.Warningf("Failed: Calling openstack AuthenticateV3: %v", err)
		return nil, false, err
	}

	providerUserID := username
	if a.useKeystoneIdentity {
		providerUserID = userid
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, providerUserID)
	identity.Extra[authapi.IdentityPreferredUsernameKey] = username

	return identitymapper.UserFor(a.identityMapper, identity)
}
