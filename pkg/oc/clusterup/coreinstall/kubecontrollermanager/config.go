package kubecontrollermanager

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/fileutil"
	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

func MakeOpenShiftAPIServerConfig(existingMasterConfig string, basedir string) (string, error) {
	configDir := path.Join(basedir, "kube-controller-manager")
	glog.V(1).Infof("Copying kube-controller-manager config to local directory %s", configDir)
	err := fileutil.CopyDirectory(existingMasterConfig, configDir)
	if err != nil {
		return "", err
	}
	os.Remove(path.Join(configDir, "master-config.yaml"))

	glog.Infof("Copying kube-controller-manager config to local directory %s", path.Join(configDir, "config.yaml"))
	return configDir, ioutil.WriteFile(path.Join(configDir, "config.yaml"), manifests.MustAsset("install/kube-controller-manager/config.yaml"), 0644)
}
