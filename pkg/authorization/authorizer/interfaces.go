package authorizer

import (
	"net/http"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
)

type Authorizer interface {
	Authorize(a AuthorizationAttributes) (allowed bool, reason string, err error)
	GetAllowedSubjects(attributes AuthorizationAttributes) ([]string, []string, error)
}

type AuthorizationAttributeBuilder interface {
	GetAttributes(request *http.Request) (AuthorizationAttributes, error)
}

type AuthorizationAttributes interface {
	GetUserInfo() user.Info
	GetVerb() string
	GetResource() string
	GetNamespace() string
	GetResourceName() string
	// GetRequestAttributes is of type interface{} because different verbs and different Authorizer/AuthorizationAttributeBuilder pairs may have different contract requirements
	GetRequestAttributes() interface{}
}
