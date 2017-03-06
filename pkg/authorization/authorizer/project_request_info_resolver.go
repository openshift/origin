package authorizer

import (
	"net/http"

	"k8s.io/kubernetes/pkg/apiserver/request"
)

type projectRequestInfoResolver struct {
	// infoFactory is used to determine info for the request
	infoFactory RequestInfoFactory
}

func NewProjectRequestInfoResolver(infoFactory RequestInfoFactory) RequestInfoFactory {
	return &projectRequestInfoResolver{
		infoFactory: infoFactory,
	}
}

func (a *projectRequestInfoResolver) NewRequestInfo(req *http.Request) (*request.RequestInfo, error) {
	requestInfo, err := a.infoFactory.NewRequestInfo(req)
	if err != nil {
		return requestInfo, err
	}

	// if the resource is projects, we need to set the namespace to the value of the name.
	if (requestInfo.Resource == "projects") && (len(requestInfo.Name) > 0) {
		requestInfo.Namespace = requestInfo.Name
	}

	return requestInfo, nil
}
