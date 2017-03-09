package authorizer

import (
	"errors"
	"net/http"

	"k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

type openshiftAuthorizationAttributeBuilder struct {
	contextMapper apirequest.RequestContextMapper
	infoFactory   RequestInfoFactory
}

func NewAuthorizationAttributeBuilder(contextMapper apirequest.RequestContextMapper, infoFactory RequestInfoFactory) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{contextMapper, infoFactory}
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (authorizer.Attributes, error) {

	ctx, ok := a.contextMapper.Get(req)
	if !ok {
		return nil, errors.New("no context found for request")
	}

	user, ok := apirequest.UserFrom(ctx)
	if !ok {
		return nil, errors.New("no user found on context")
	}

	requestInfo, err := a.infoFactory.NewRequestInfo(req)
	if err != nil {
		return nil, err
	}

	attribs := authorizer.AttributesRecord{
		User: user,

		ResourceRequest: requestInfo.IsResourceRequest,
		Path:            requestInfo.Path,
		Verb:            requestInfo.Verb,

		APIGroup:    requestInfo.APIGroup,
		APIVersion:  requestInfo.APIVersion,
		Resource:    requestInfo.Resource,
		Subresource: requestInfo.Subresource,
		Namespace:   requestInfo.Namespace,
		Name:        requestInfo.Name,
	}

	return attribs, nil
}
