package policy

import (
	"fmt"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// NoBuildConfigLabelError represents an error caused by the build not having
// the required build config label.
type NoBuildConfigLabelError struct {
	build *buildapi.Build
}

func NewNoBuildConfigLabelError(build *buildapi.Build) error {
	return NoBuildConfigLabelError{build: build}
}

func (e NoBuildConfigLabelError) Error() string {
	return fmt.Sprintf("build %s/%s does not have required %q label set", e.build.Namespace, e.build.Name, buildapi.BuildConfigLabel)
}

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
