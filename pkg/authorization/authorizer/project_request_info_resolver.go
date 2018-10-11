package authorizer

import (
	"net/http"

	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/openshift/origin/pkg/project/apis/project"
)

type projectRequestInfoResolver struct {
	// infoFactory is used to determine info for the request
	infoFactory apirequest.RequestInfoResolver
}

func NewProjectRequestInfoResolver(infoFactory apirequest.RequestInfoResolver) apirequest.RequestInfoResolver {
	return &projectRequestInfoResolver{
		infoFactory: infoFactory,
	}
}

func (a *projectRequestInfoResolver) NewRequestInfo(req *http.Request) (*apirequest.RequestInfo, error) {
	requestInfo, err := a.infoFactory.NewRequestInfo(req)
	if err != nil {
		return requestInfo, err
	}

	// if the resource is projects, we need to set the namespace to the value of the name.
	if (len(requestInfo.APIGroup) == 0 || requestInfo.APIGroup == project.GroupName) && requestInfo.Resource == "projects" && len(requestInfo.Name) > 0 {
		requestInfo.Namespace = requestInfo.Name
	}

	return requestInfo, nil
}
