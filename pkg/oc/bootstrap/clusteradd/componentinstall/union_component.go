package componentinstall

import "github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"

type Components []Component

func (c Components) Name() string {
	return "union"
}
func (c Components) Install(dockerClient dockerhelper.Interface) error {
	return InstallComponents(c, dockerClient)
}
