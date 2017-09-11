package authorizer

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

type SubjectLocator interface {
	GetAllowedSubjects(attributes authorizer.Attributes) (sets.String, sets.String, error)
}

// ForbiddenMessageMaker creates a forbidden message from Attributes
type ForbiddenMessageMaker interface {
	MakeMessage(attrs authorizer.Attributes) (string, error)
}
