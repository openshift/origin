package kubeapiserver

import (
	"os"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/golang/glog"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"

	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/run"
	"github.com/openshift/origin/pkg/oc/lib/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

const KubeAPIServerDirName = "kube-apiserver"
const OpenShiftAPIServerDirName = "openshift-apiserver"
const OpenShiftControllerManagerDirName = "openshift-controller-manager"

type KubeAPIServerStartConfig struct {
	// MasterImage is the docker image for openshift start master
	MasterImage string

	Args []string
}

func NewKubeAPIServerStartConfig() *KubeAPIServerStartConfig {
	return &KubeAPIServerStartConfig{}

}

// Start starts the OpenShift master as a Docker container
// and returns a directory in the local file system where
// the OpenShift configuration has been copied
func (opt KubeAPIServerStartConfig) MakeMasterConfig(dockerClient dockerhelper.Interface, basedir string) (string, error) {
	componentName := "create-master-config"
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	glog.Infof("Running %q", componentName)

	createConfigCmd := []string{
		"start", "master",
	}
	createConfigCmd = append(createConfigCmd, opt.Args...)

	containerId, rc, err := imageRunHelper.Image(opt.MasterImage).
		Privileged().
		HostNetwork().
		HostPid().
		SaveContainerLogs(componentName, path.Join(basedir, "logs")).
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

	// TODO eliminate the linkage that other tasks have on this particular structure
	masterDir := path.Join(basedir, KubeAPIServerDirName)
	if err := os.MkdirAll(masterDir, 0755); err != nil {
		return "", err
	}
	glog.V(1).Infof("Copying OpenShift config to local directory %s", masterDir)
	if err = dockerhelper.DownloadDirFromContainer(dockerClient, containerId, "/var/lib/origin/openshift.local.config", masterDir); err != nil {
		if removeErr := os.RemoveAll(masterDir); removeErr != nil {
			glog.V(2).Infof("Error removing temporary config dir %s: %v", masterDir, removeErr)
		}
		return "", err
	}

	// update some listen information to include starting the DNS server
	masterconfigFilename := path.Join(masterDir, "master-config.yaml")
	masterconfig, err := componentinstall.ReadMasterConfig(masterconfigFilename)
	if err != nil {
		return "", err
	}

	addImagePolicyAdmission(&masterconfig.AdmissionConfig)

	if err := componentinstall.WriteMasterConfig(masterconfigFilename, masterconfig); err != nil {
		return "", err
	}

	return masterDir, nil
}

func addImagePolicyAdmission(admissionConfig *configapi.AdmissionConfig) {
	// default openshift image policy admission
	if admissionConfig.PluginConfig == nil {
		admissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{}
	}
	// Add default ImagePolicyConfig into openshift api master config
	policyConfig := []byte(`{"kind":"ImagePolicyConfig","apiVersion":"v1","executionRules":[{"name":"execution-denied",
"onResources":[{"resource":"pods"},{"resource":"builds"}],"reject":true,"matchImageAnnotations":[{"key":"images.openshift.io/deny-execution",
"value":"true"}],"skipOnResolutionFailure":true}]}`)
	admissionConfig.PluginConfig["openshift.io/ImagePolicy"] = &configapi.AdmissionPluginConfig{
		Configuration: runtime.RawExtension{Raw: policyConfig},
	}
}
