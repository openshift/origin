package componentinstall

import "github.com/openshift/origin/pkg/oc/clusterup/docker/util"

type Components []Component

func (c Components) Name() string {
	return "union"
}
func (c Components) Install(dockerClient util.Interface) error {
	return InstallComponents(c, dockerClient)
}
