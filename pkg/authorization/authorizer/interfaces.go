package authorizer

import (
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// ForbiddenMessageMaker creates a forbidden message from Attributes
type ForbiddenMessageMaker interface {
	MakeMessage(attrs authorizer.Attributes) (string, error)
}
