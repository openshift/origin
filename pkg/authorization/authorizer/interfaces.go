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

// ForbiddenMessageMaker creates a forbidden message from a MessageContext
type ForbiddenMessageMaker interface {
	MakeMessage(ctx MessageContext) (string, error)
}

// MessageContext contains sufficient information to create a forbidden message.  It is bundled in this one object to make it easy and obvious how to build a golang template
type MessageContext struct {
	Attributes authorizer.Attributes
}
