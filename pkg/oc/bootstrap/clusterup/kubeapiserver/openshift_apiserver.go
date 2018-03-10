package kubeapiserver

import (
	"io/ioutil"
	"path"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/golang/glog"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/tmpformac"
)

func MakeOpenShiftAPIServerConfig(existingMasterConfig string, basedir string) (string, error) {
	openshiftAPIServerDir := path.Join(basedir, OpenShiftAPIServerDirName)
	glog.V(1).Infof("Copying kube-apiserver config to local directory %s", openshiftAPIServerDir)
	if err := tmpformac.CopyDirectory(existingMasterConfig, openshiftAPIServerDir); err != nil {
		return "", err
	}

	// update some listen information to include starting the DNS server
	masterconfigFilename := path.Join(openshiftAPIServerDir, "master-config.yaml")
	originalBytes, err := ioutil.ReadFile(masterconfigFilename)
	if err != nil {
		return "", err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, originalBytes)
	if err != nil {
		return "", err
	}
	masterconfig := configObj.(*configapi.MasterConfig)
	masterconfig.ServingInfo.BindAddress = "0.0.0.0:8445"

	configBytes, err := runtime.Encode(configapilatest.Codec, masterconfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(masterconfigFilename, configBytes, 0644); err != nil {
		return "", err
	}

	return openshiftAPIServerDir, nil
}
