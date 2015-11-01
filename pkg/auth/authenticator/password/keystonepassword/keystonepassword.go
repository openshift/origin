package keystonepassword

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/golang/glog"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util"
)

// keystonePasswordAuthenticator uses OpenStack keystone to authenticate a user by password
type keystonePasswordAuthenticator struct {
	providerName   string
	url            string
	client         *http.Client
	domainName     string
	identityMapper authapi.UserIdentityMapper
}

// New creates a new password authenticator that uses OpenStack keystone to authenticate a user by password
// A custom transport can be provided (typically to customize TLS options like trusted roots or present a client certificate).
// If no transport is provided, http.DefaultTransport is used
func New(providerName string, url string, transport http.RoundTripper, domainName string, identityMapper authapi.UserIdentityMapper) authenticator.Password {
	if transport == nil {
		transport = http.DefaultTransport
	}
	client := &http.Client{Transport: transport}
	return &keystonePasswordAuthenticator{providerName, url, client, domainName, identityMapper}
}

// AuthenticatePassword approves any login attempt which is successfully validated with Keystone
func (a keystonePasswordAuthenticator) AuthenticatePassword(username, password string) (user.Info, bool, error) {
	defer func() {
		if e := recover(); e != nil {
			util.HandleError(fmt.Errorf("recovered panic: %v, %s", e, debug.Stack()))
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
	err = openstack.AuthenticateV3(client, opts)
	if err != nil {
		if responseErr, ok := err.(*gophercloud.UnexpectedResponseCodeError); ok {
			if responseErr.Actual == 401 {
				return nil, false, nil
			}
		}
		glog.Warningf("Failed: Calling openstack AuthenticateV3: %v", err)
		return nil, false, err
	}

	identity := authapi.NewDefaultUserIdentityInfo(a.providerName, username)
	user, err := a.identityMapper.UserFor(identity)
	if err != nil {
		return nil, false, fmt.Errorf("error creating or updating mapping for: %#v due to %v", identity, err)
	}
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)

	return user, true, nil
}
