package kubeapiserver

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/tmpformac"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

func MakeOpenShiftControllerConfig(existingMasterConfig string, basedir string) (string, error) {
	configDir := path.Join(basedir, OpenShiftControllerManagerDirName)
	glog.V(1).Infof("Copying kube-apiserver config to local directory %s", OpenShiftControllerManagerDirName)
	if err := tmpformac.CopyDirectory(existingMasterConfig, configDir); err != nil {
		return "", err
	}
	if err := os.Remove(path.Join(configDir, "master-config.yaml")); err != nil {
		return "", err
	}
	if err := os.Remove(path.Join(configDir, "config.json")); err != nil {
		return "", err
	}

	configFilename := path.Join(configDir, "config.json")
	if err := ioutil.WriteFile(configFilename, manifests.MustAsset("install/openshift-controller-manager/static-config.json"), 0644); err != nil {
		return "", err
	}

	return configDir, nil
}
