package kubeapiserver

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

type KubeAPIServerStartConfig struct {
	// MasterImage is the docker image for openshift start master
	MasterImage string
	ImageFormat string
	DNSPort     int
	PublicHost  string
}

func NewKubeAPIServerStartConfig() *KubeAPIServerStartConfig {
	return &KubeAPIServerStartConfig{}

}

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (opt KubeAPIServerStartConfig) MakeMasterConfig(dockerClient dockerhelper.Interface, imageRunHelper *run.Runner, out io.Writer) (string, error) {
	fmt.Fprintf(out, "Creating initial OpenShift master configuration\n")
	createConfigCmd := []string{
		"start", "master",
		"--write-config=/var/lib/origin/openshift.local.config",
		"--master=127.0.0.1",
		fmt.Sprintf("--images=%s", opt.ImageFormat),
		fmt.Sprintf("--dns=0.0.0.0:%d", opt.DNSPort),
		fmt.Sprintf("--public-master=https://%s:8443", opt.PublicHost),
	}
	containerId, _, err := imageRunHelper.Image(opt.MasterImage).
		Privileged().
		HostNetwork().
		HostPid().
		Command(createConfigCmd...).Run()
	if err != nil {
		return "", errors.NewError("could not create OpenShift configuration: %v", err).WithCause(err)
	}

	tempDir, err := ioutil.TempDir("", "oc-cluster-up-kube-apiserver-")
	if err != nil {
		return "", err
	}
	// TODO eliminate the linkage that other tasks have on this particular structure
	masterDir := path.Join(tempDir, "master")
	if err := os.Mkdir(masterDir, 0755); err != nil {
		return "", err
	}
	glog.V(1).Infof("Copying OpenShift config to local directory %s", tempDir)
	if err = dockerhelper.DownloadDirFromContainer(dockerClient, containerId, "/var/lib/origin/openshift.local.config", masterDir); err != nil {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			glog.V(2).Infof("Error removing temporary config dir %s: %v", tempDir, removeErr)
		}
		return "", err
	}

	return masterDir, nil
}
