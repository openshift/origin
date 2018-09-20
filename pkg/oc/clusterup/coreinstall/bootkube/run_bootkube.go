package bootkube

import (
	"fmt"
	"os"
	"path"

	"github.com/openshift/origin/pkg/oc/clusterup/docker/openshift"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/util"
	"github.com/openshift/origin/pkg/oc/lib/errors"
)

type BootkubeRunConfig struct {
	BootkubeImage string

	// StaticPodManifestDir is location where kubelet awaits the static pod manifests.
	StaticPodManifestDir string

	// AssetsDir is location where bootkube expects assets.
	AssetsDir string

	// ContainerBinds is location to additional container bind mounts for bootkube containers.
	ContainerBinds []string
}

// RemoveApiserver removes the apiserver manifest because the cluster-kube-apiserver-operator will generate them.
// Eventually, our operators will generate all files and we don't need bootkube render anymore.
func (opt *BootkubeRunConfig) RemoveApiserver(kubernetesDir string) error {
	os.Remove(path.Join(kubernetesDir, "bootstrap-manifests", "bootstrap-apiserver.yaml"))
	os.Remove(path.Join(kubernetesDir, "manifests", "kube-apiserver.yaml"))

	return nil
}

// RunStart runs the bootkube start command. The AssetsDir have to be specified as well as the StaticPodManifestDir.
func (opt *BootkubeRunConfig) RunStart(dockerClient util.Interface) (string, error) {
	imageRunHelper := run.NewRunHelper(util.NewHelper(dockerClient)).New()

	startCommand := []string{
		"start",
		"--pod-manifest-path=/etc/kubernetes/manifests",
		"--asset-dir=/assets",
		"--strict",
	}

	binds := opt.ContainerBinds
	binds = append(binds, fmt.Sprintf("%s:/assets:z", opt.AssetsDir))

	containerID, exitCode, err := imageRunHelper.Image(opt.BootkubeImage).
		Name(openshift.BootkubeStartContainerName).
		DiscardContainer().
		HostNetwork().
		HostPid().
		Bind(binds...).
		Entrypoint("/bootkube").
		Command(startCommand...).Run()

	if err != nil {
		return "", errors.NewError("bootkube start exited %d: %v", exitCode, err).WithCause(err)
	}

	return containerID, nil
}
