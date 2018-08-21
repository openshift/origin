package policy

import (
	"fmt"

	buildv1 "github.com/openshift/api/build/v1"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

// NoBuildNumberLabelError represents an error caused by the build not having
// the required build number annotation.
type NoBuildNumberAnnotationError struct {
	build *buildv1.Build
}

func NewNoBuildNumberAnnotationError(build *buildv1.Build) error {
	return NoBuildNumberAnnotationError{build: build}
}

func (e NoBuildNumberAnnotationError) Error() string {
	return fmt.Sprintf("build %s/%s does not have required %q annotation set", e.build.Namespace, e.build.Name, buildutil.BuildNumberAnnotation)
}
