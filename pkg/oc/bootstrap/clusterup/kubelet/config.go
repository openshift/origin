package kubelet

import (
	"fmt"
	"os"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

const (
	NodeConfigDirName  = "node"
	KubeDNSDirName     = "kubedns"
	PodManifestDirName = "static-pod-manifests"
)

type NodeStartConfig struct {
	// ContainerBinds is a list of local/path:image/path pairs
	ContainerBinds []string
	// NodeImage is the docker image for the openshift cli
	CLIImage string
	// NodeImage is the docker image for openshift start node
	NodeImage string

	Args []string
}

func NewNodeStartConfig() *NodeStartConfig {
	return &NodeStartConfig{
		ContainerBinds: []string{},
	}

}

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (opt NodeStartConfig) MakeNodeConfig(dockerClient dockerhelper.Interface, basedir string) (string, error) {
	componentName := "create-node-config"
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	glog.Infof("Running %q", componentName)

	createConfigCmd := []string{
		"adm", "create-node-config",
		fmt.Sprintf("--node-dir=%s", "/var/lib/origin/openshift.local.config"),
	}
	createConfigCmd = append(createConfigCmd, opt.Args...)

	containerId, rc, err := imageRunHelper.Image(opt.CLIImage).
		Privileged().
		HostNetwork().
		HostPid().
		Bind(opt.ContainerBinds...).
		SaveContainerLogs(componentName, path.Join(basedir, "logs")).
		Entrypoint("oc").
		Command(createConfigCmd...).Run()
	defer func() {
		if err = dockerClient.ContainerRemove(containerId, types.ContainerRemoveOptions{}); err != nil {
			glog.Errorf("error removing %q: %v", containerId, err)
		}
	}()
	if err != nil {
		return "", errors.NewError("could not run %q: %v", componentName, err).WithCause(err)
	}
	if rc != 0 {
		return "", errors.NewError("could not run %q: rc==%v", componentName, rc)
	}

	nodeConfigDir := path.Join(basedir, NodeConfigDirName)
	glog.V(1).Infof("Copying OpenShift node config to local directory %s", nodeConfigDir)
	if err = dockerhelper.DownloadDirFromContainer(dockerClient, containerId, "/var/lib/origin/openshift.local.config", nodeConfigDir); err != nil {
		if removeErr := os.RemoveAll(nodeConfigDir); removeErr != nil {
			glog.V(2).Infof("Error removing temporary config dir %s: %v", nodeConfigDir, removeErr)
		}
		return "", err
	}

	return nodeConfigDir, nil
}
