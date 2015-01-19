package app

import (
	"github.com/fsouza/go-dockerclient"
	"strings"
)

var stiEnvironmentNames = []string{"STI_LOCATION", "STI_SCRIPTS_URL", "STI_BUILDER"}

func IsBuilderImage(image *docker.Image) bool {
	for _, env := range image.Config.Env {
		for _, name := range stiEnvironmentNames {
			if strings.HasPrefix(env, name+"=") {
				return true
			}
		}
	}
	return false
}
