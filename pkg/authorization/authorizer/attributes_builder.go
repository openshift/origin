package authorizer

import (
	"errors"
	"net/http"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/authorizer"
)

type openshiftAuthorizationAttributeBuilder struct {
	contextMapper kapi.RequestContextMapper
	infoFactory   RequestInfoFactory
}

func NewAuthorizationAttributeBuilder(contextMapper kapi.RequestContextMapper, infoFactory RequestInfoFactory) AuthorizationAttributeBuilder {
	return &openshiftAuthorizationAttributeBuilder{contextMapper, infoFactory}
}

func (a *openshiftAuthorizationAttributeBuilder) GetAttributes(req *http.Request) (authorizer.Attributes, error) {

	ctx, ok := a.contextMapper.Get(req)
	if !ok {
		return nil, errors.New("no context found for request")
	}

	user, ok := kapi.UserFrom(ctx)
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
