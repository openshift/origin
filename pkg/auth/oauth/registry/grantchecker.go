package registry

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/origin/pkg/auth/api"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	"github.com/openshift/origin/pkg/oauth/scope"
)

type ClientAuthorizationGrantChecker struct {
	client oauthclient.OAuthClientAuthorizationInterface
}

func NewClientAuthorizationGrantChecker(client oauthclient.OAuthClientAuthorizationInterface) *ClientAuthorizationGrantChecker {
	return &ClientAuthorizationGrantChecker{client}
}

func (c *ClientAuthorizationGrantChecker) HasAuthorizedClient(user user.Info, grant *api.Grant) (approved bool, err error) {
	id := oauthclientauthorization.ClientAuthorizationName(user.GetName(), grant.Client.GetId())
	authorization, err := c.client.Get(id, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(authorization.UserUID) != 0 && authorization.UserUID != user.GetUID() {
		return false, fmt.Errorf("user %s UID %s does not match stored client authorization value for UID %s", user.GetName(), user.GetUID(), authorization.UserUID)
	}
	// TODO: improve this to allow the scope implementation to determine overlap
	if !scope.Covers(authorization.Scopes, scope.Split(grant.Scope)) {
		return false, nil
	}
	return true, nil
}
