package builder

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/build/api"
)

type Builder interface {
	Build() error
}

// imageTag returns the tag to be used for the build. If a registry has been
// specified, it will prepend the registry to the name
func imageTag(build *api.Build) string {
	tag := build.Parameters.Output.ImageTag
	if !strings.HasPrefix(tag, build.Parameters.Output.Registry) {
		tag = fmt.Sprintf("%s/%s", build.Parameters.Output.Registry, tag)
	}
	return tag
}

// getBuildEnvVars returns a map with the environment variables that should be added
// to the built image
func getBuildEnvVars(build *api.Build) map[string]string {
	envVars := map[string]string{
		"OPENSHIFT_BUILD_NAME":   build.Name,
		"OPENSHIFT_BUILD_SOURCE": build.Parameters.Source.Git.URI,
	}
	if build.Parameters.Source.Git.Ref != "" {
		envVars["OPENSHIFT_BUILD_REFERENCE"] = build.Parameters.Source.Git.Ref
	}
	if build.Parameters.Revision != nil &&
		build.Parameters.Revision.Git != nil &&
		build.Parameters.Revision.Git.Commit != "" {
		envVars["OPENSHIFT_BUILD_COMMIT"] = build.Parameters.Revision.Git.Commit
	}
	return envVars
}
