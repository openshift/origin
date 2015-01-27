package app

import (
	"strings"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

var stiEnvironmentNames = []string{"STI_LOCATION", "STI_SCRIPTS_URL", "STI_BUILDER"}

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
