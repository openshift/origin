package authorizer

import (
	"net/http"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
)

type openshiftAuthorizationAttributeBuilder struct {
	contextMapper kapi.RequestContextMapper
	infoFactory   RequestInfoFactory
}

func NewAuthorizationAttributeBuilder(contextMapper kapi.RequestContextMapper, infoFactory RequestInfoFactory) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{contextMapper, infoFactory}
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (Action, error) {
	requestInfo, err := a.infoFactory.NewRequestInfo(req)
	if err != nil {
		return nil, err
	}

	if !requestInfo.IsResourceRequest {
		return DefaultAuthorizationAttributes{
			Verb:           strings.ToLower(req.Method),
			NonResourceURL: true,
			URL:            requestInfo.Path,
		}, nil
	}

	resource := requestInfo.Resource
	if len(requestInfo.Subresource) > 0 {
		resource = requestInfo.Resource + "/" + requestInfo.Subresource
	}

	return DefaultAuthorizationAttributes{
		Verb:              requestInfo.Verb,
		APIGroup:          requestInfo.APIGroup,
		APIVersion:        requestInfo.APIVersion,
		Resource:          resource,
		ResourceName:      requestInfo.Name,
		RequestAttributes: req,
		NonResourceURL:    false,
		URL:               requestInfo.Path,
	}, nil
}
