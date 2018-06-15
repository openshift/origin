package policy

import (
	"fmt"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

// NoBuildNumberLabelError represents an error caused by the build not having
// the required build number annotation.
type NoBuildNumberAnnotationError struct {
	build *buildapi.Build
}

func NewNoBuildNumberAnnotationError(build *buildapi.Build) error {
	return NoBuildNumberAnnotationError{build: build}
}

func (e NoBuildNumberAnnotationError) Error() string {
	return fmt.Sprintf("build %s/%s does not have required %q annotation set", e.build.Namespace, e.build.Name, buildapi.BuildNumberAnnotation)
}
