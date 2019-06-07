package server

import (
	"flag"

	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
	"k8s.io/apimachinery/pkg/util/wait"
)

func StartConfiguredMaster(masterConfig *configapi.MasterConfig) (string, error) {
	if v := flag.Lookup("test.v"); v == nil {
		panic("cannot be used outside of test code")
	}

	// this is for testing. TODO: update the billion call sites with proper stopCh setup code
	return StartConfiguredMasterWithOptions(masterConfig, wait.NeverStop)
}

func StartConfiguredMasterAPI(masterConfig *configapi.MasterConfig) (string, error) {
	if v := flag.Lookup("test.v"); v == nil {
		panic("cannot be used outside of test code")
	}

	// we need to unconditionally start this controller for rbac permissions to work
	if masterConfig.KubernetesMasterConfig.ControllerArguments == nil {
		masterConfig.KubernetesMasterConfig.ControllerArguments = map[string][]string{}
	}
	masterConfig.KubernetesMasterConfig.ControllerArguments["controllers"] = append(masterConfig.KubernetesMasterConfig.ControllerArguments["controllers"], "serviceaccount-token", "clusterrole-aggregation")

	// this is for testing. TODO: update the billion call sites with proper stopCh setup code
	return StartConfiguredMasterWithOptions(masterConfig, wait.NeverStop)
}

// StartTestMaster starts up a test master and returns back the startOptions so you can get clients and certs
func StartTestMaster() (*configapi.MasterConfig, string, error) {
	master, err := DefaultMasterOptions()
	if err != nil {
		return nil, "", err
	}

	adminKubeConfigFile, err := StartConfiguredMaster(master)
	return master, adminKubeConfigFile, err
}

func StartTestMasterAPI() (*configapi.MasterConfig, string, error) {
	master, err := DefaultMasterOptions()
	if err != nil {
		return nil, "", err
	}

	adminKubeConfigFile, err := StartConfiguredMasterAPI(master)
	return master, adminKubeConfigFile, err
}
