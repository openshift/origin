package authorizer

import (
	"net/http"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

type SubjectLocator interface {
	GetAllowedSubjects(attributes authorizer.Attributes) (sets.String, sets.String, error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (authorizer.Attributes, error)
}

type RequestInfoFactory interface {
	NewRequestInfo(req *http.Request) (*apirequest.RequestInfo, error)
}

// ForbiddenMessageMaker creates a forbidden message from Attributes
type ForbiddenMessageMaker interface {
	MakeMessage(attrs authorizer.Attributes) (string, error)
}
