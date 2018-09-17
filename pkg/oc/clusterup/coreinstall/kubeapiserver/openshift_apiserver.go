package kubeapiserver

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/tmpformac"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

func MakeOpenShiftAPIServerConfig(existingMasterConfig string, basedir string) (string, error) {
	configDir := path.Join(basedir, OpenShiftAPIServerDirName)
	glog.V(1).Infof("Copying kube-apiserver config to local directory %s", configDir)
	err := tmpformac.CopyDirectory(existingMasterConfig, configDir)
	if err != nil {
		return "", err
	}
	if err := os.Remove(path.Join(configDir, "master-config.yaml")); err != nil {
		return "", err
	}

	masterConfigFilename := path.Join(configDir, "config.json")
	if err := ioutil.WriteFile(masterConfigFilename, manifests.MustAsset("install/openshift-apiserver/static-config.json"), 0644); err != nil {
		return "", err
	}

	return configDir, nil
}
