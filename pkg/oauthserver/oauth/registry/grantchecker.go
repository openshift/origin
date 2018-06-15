package registry

import (
	stderrors "errors"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/util/retry"

	oauth "github.com/openshift/api/oauth/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	"github.com/openshift/origin/pkg/oauth/scope"
	"github.com/openshift/origin/pkg/oauthserver/api"

	"github.com/golang/glog"
)

var errEmptyUID = stderrors.New("user from request has empty UID and thus cannot perform a grant flow")

type ClientAuthorizationGrantChecker struct {
	client oauthclient.OAuthClientAuthorizationInterface
}

func NewClientAuthorizationGrantChecker(client oauthclient.OAuthClientAuthorizationInterface) *ClientAuthorizationGrantChecker {
	return &ClientAuthorizationGrantChecker{client}
}

func (c *ClientAuthorizationGrantChecker) HasAuthorizedClient(user kuser.Info, grant *api.Grant) (approved bool, err error) {
	// Validation prevents OAuthClientAuthorization.UserUID from being empty (and always has).
	// However, user.GetUID() is empty during impersonation, meaning this flow does not work for impersonation.
	// This is fine because no OAuth / grant flow works with impersonation in general.
	if len(user.GetUID()) == 0 {
		return false, errEmptyUID
	}

	id := oauthclientauthorization.ClientAuthorizationName(user.GetName(), grant.Client.GetId())
	var authorization *oauth.OAuthClientAuthorization

	// getClientAuthorization ignores not found errors, thus it is possible for authorization to be nil
	// getClientAuthorization does not ignore conflict errors, so we retry those in case we having multiple clients racing on this grant flow
	// all other errors are considered fatal
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		authorization, err = c.getClientAuthorization(id, user)
		return err
	}); err != nil {
		return false, err
	}

	// TODO: improve this to allow the scope implementation to determine overlap
	if authorization == nil || !scope.Covers(authorization.Scopes, scope.Split(grant.Scope)) {
		return false, nil
	}

	return true, nil
}

// getClientAuthorization gets the OAuthClientAuthorization with the given name and validates that it matches the given user
// it attempts to delete stale client authorizations, and thus must be retried in case of conflicts
func (c *ClientAuthorizationGrantChecker) getClientAuthorization(name string, user kuser.Info) (*oauth.OAuthClientAuthorization, error) {
	authorization, err := c.client.Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// if no such authorization exists, it simply means the user needs to go through the grant flow
		return nil, nil
	}
	if err != nil {
		// any other error is fatal (this will never be retried since it cannot return a conflict error)
		return nil, err
	}

	// check to see if we have a stale authorization
	// user.GetUID() and authorization.UserUID are both guaranteed to be non-empty
	if user.GetUID() != authorization.UserUID {
		glog.Infof("%#v does not match stored client authorization %#v, attempting to delete stale authorization", user, authorization)
		if err := c.client.Delete(name, metav1.NewPreconditionDeleteOptions(string(authorization.UID))); err != nil && !errors.IsNotFound(err) {
			// ignore not found since that could be caused by multiple grant flows occurring at once (the other flow deleted the authorization before we did)
			// this could be a conflict error, which will cause this whole function to be retried
			return nil, err
		}
		// we successfully deleted the authorization so the user needs to go through the grant flow
		return nil, nil
	}

	// everything looks good so we can return the authorization
	return authorization, nil
}
