package registry

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	"github.com/openshift/origin/pkg/oauth/scope"
	"k8s.io/apiserver/pkg/authentication/user"
)

type ClientAuthorizationGrantChecker struct {
	registry oauthclientauthorization.Registry
}

func NewClientAuthorizationGrantChecker(registry oauthclientauthorization.Registry) *ClientAuthorizationGrantChecker {
	return &ClientAuthorizationGrantChecker{registry}
}

func (c *ClientAuthorizationGrantChecker) HasAuthorizedClient(user user.Info, grant *api.Grant) (approved bool, err error) {
	id := c.registry.ClientAuthorizationName(user.GetName(), grant.Client.GetId())
	authorization, err := c.registry.GetClientAuthorization(apirequest.NewContext(), id, &metav1.GetOptions{})
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
