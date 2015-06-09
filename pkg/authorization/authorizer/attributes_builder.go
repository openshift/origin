package authorizer

import (
	"net/http"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapiserver "github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
)

type openshiftAuthorizationAttributeBuilder struct {
	contextMapper kapi.RequestContextMapper
	infoResolver  *kapiserver.APIRequestInfoResolver
}

func NewAuthorizationAttributeBuilder(contextMapper kapi.RequestContextMapper, infoResolver *kapiserver.APIRequestInfoResolver) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{contextMapper, infoResolver}
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (AuthorizationAttributes, error) {
	// any url that starts with an API prefix and is more than one step long is considered to be a resource URL.
	// That means that /api is non-resource, /api/v1 is resource, /healthz is non-resource, and /swagger/anything is non-resource
	urlSegments := splitPath(req.URL.Path)
	isResourceURL := (len(urlSegments) > 1) && a.infoResolver.APIPrefixes.Has(urlSegments[0])

	if !isResourceURL {
		return DefaultAuthorizationAttributes{
			Verb:           strings.ToLower(req.Method),
			NonResourceURL: true,
			URL:            req.URL.Path,
		}, nil
	}

	requestInfo, err := a.infoResolver.GetAPIRequestInfo(req)
	if err != nil {
		return nil, err
	}

	resource := requestInfo.Resource
	if len(requestInfo.Subresource) > 0 {
		resource = requestInfo.Resource + "/" + requestInfo.Subresource
	}

	return DefaultAuthorizationAttributes{
		Verb:              requestInfo.Verb,
		APIVersion:        requestInfo.APIVersion,
		Resource:          resource,
		ResourceName:      requestInfo.Name,
		RequestAttributes: req,
		NonResourceURL:    false,
		URL:               req.URL.Path,
	}, nil
}
