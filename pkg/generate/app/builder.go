package app

import (
	"strings"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

var stiEnvironmentNames = []string{"STI_LOCATION", "STI_SCRIPTS_URL", "STI_BUILDER"}

// IsBuilderImage checks whether the provided Docker image is
// a builder image or not
func IsBuilderImage(image *imageapi.DockerImage) bool {
	for _, env := range image.Config.Env {
		for _, name := range stiEnvironmentNames {
			if strings.HasPrefix(env, name+"=") {
				return true
			}
		}
	}
	return false
}

// BuilderForPlatform expands the provided platform to
// the respective OpenShift image
//
//TODO: Remove once a real image searcher is implemented
func BuilderForPlatform(platform string) string {
	switch strings.ToLower(platform) {
	case "ruby":
		return "openshift/ruby-20-centos7"
	case "jee":
		return "openshift/wildfly-8-centos"
	case "nodejs":
		return "openshift/nodejs-010-centos7"
	}
	return ""
}
