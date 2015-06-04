package authorizer

import (
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func IsVerbAllowedOnBaseResource(ctx kapi.Context, authorizer Authorizer, restriction *authorizationapi.IsVerbAllowedOnBaseResource, a AuthorizationAttributes) (bool, error) {

	attributes := DefaultAuthorizationAttributes{
		Verb:              restriction.Verb,
		APIVersion:        a.GetAPIVersion(),
		Resource:          strings.Split(a.GetResource(), "/")[0], // get to the base resource (eliminate any subresources)
		ResourceName:      a.GetResourceName(),
		RequestAttributes: a.GetRequestAttributes(),
		NonResourceURL:    a.IsNonResourceURL(),
		URL:               a.GetURL(),
	}

	allowed, _, err := authorizer.Authorize(ctx, attributes)
	return allowed, err
}
