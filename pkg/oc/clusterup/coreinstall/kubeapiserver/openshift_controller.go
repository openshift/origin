package kubeapiserver

import (
	"path"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/tmpformac"
)

func MakeOpenShiftControllerConfig(existingMasterConfig string, basedir string) (string, error) {
	configDir := path.Join(basedir, OpenShiftControllerManagerDirName)
	glog.V(1).Infof("Copying kube-apiserver config to local directory %s", OpenShiftControllerManagerDirName)
	if err := tmpformac.CopyDirectory(existingMasterConfig, configDir); err != nil {
		return "", err
	}

	// update some listen information to include starting the DNS server
	masterconfigFilename := path.Join(configDir, "master-config.yaml")
	masterconfig, err := componentinstall.ReadMasterConfig(masterconfigFilename)
	if err != nil {
		return "", err
	}
	masterconfig.ServingInfo.BindAddress = "0.0.0.0:8444"

	// disable the service serving cert signer because that runs in a separate pod now
	masterconfig.ControllerConfig.Controllers = []string{
		"*",
		"-openshift.io/service-serving-cert",
	}

	if err := componentinstall.WriteMasterConfig(masterconfigFilename, masterconfig); err != nil {
		return "", err
	}

	return configDir, nil
}
