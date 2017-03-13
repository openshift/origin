package authorizer

import (
	"net/http"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

type Authorizer interface {
	Authorize(ctx apirequest.Context, a Action) (allowed bool, reason string, err error)
	GetAllowedSubjects(ctx apirequest.Context, attributes Action) (sets.String, sets.String, error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (Action, error)
}

type RequestInfoFactory interface {
	NewRequestInfo(req *http.Request) (*apirequest.RequestInfo, error)
}

type Action interface {
	GetVerb() string
	GetAPIVersion() string
	GetAPIGroup() string
	// GetResource returns the resource type.  If IsNonResourceURL() is true, then GetResource() is "".
	GetResource() string
	GetResourceName() string
	// GetRequestAttributes is of type interface{} because different verbs and different Authorizer/AuthorizationAttributeBuilder pairs may have different contract requirements.
	GetRequestAttributes() interface{}
	// IsNonResourceURL returns true if this is not an action performed against the resource API
	IsNonResourceURL() bool
	// GetURL returns the URL path being requested, including the leading '/'
	GetURL() string
}

// ForbiddenMessageMaker creates a forbidden message from a MessageContext
type ForbiddenMessageMaker interface {
	MakeMessage(ctx MessageContext) (string, error)
}

// MessageContext contains sufficient information to create a forbidden message.  It is bundled in this one object to make it easy and obvious how to build a golang template
type MessageContext struct {
	User       user.Info
	Namespace  string
	Attributes Action
}
