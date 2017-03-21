package authorizer

import (
	"net/http"

	"k8s.io/kubernetes/pkg/apiserver/request"
	"k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/util/sets"
)

type SubjectLocator interface {
	GetAllowedSubjects(attributes authorizer.Attributes) (sets.String, sets.String, error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (authorizer.Attributes, error)
}

type RequestInfoFactory interface {
	NewRequestInfo(req *http.Request) (*request.RequestInfo, error)
}

// ForbiddenMessageMaker creates a forbidden message from Attributes
type ForbiddenMessageMaker interface {
	MakeMessage(attrs authorizer.Attributes) (string, error)
}
