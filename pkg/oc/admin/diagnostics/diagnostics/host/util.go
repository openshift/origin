package host

import (
	"fmt"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
)

// this would be bad practice if there were ever a need to load more than one master config for diagnostics.
// however we will proceed with the assumption that will never be necessary.
var (
	masterConfigLoaded    = false
	masterConfig          *configapi.MasterConfig
	masterConfigLoadError error
)

func GetMasterConfig(masterConfigFile string, logger *log.Logger) (*configapi.MasterConfig, error) {
	if masterConfigLoaded { // no need to do this more than once
		if masterConfigLoadError != nil {
			printMasterConfigLoadError(masterConfigFile, logger)
		}
		return masterConfig, masterConfigLoadError
	}
	logger.Debug("DH0001", fmt.Sprintf("Looking for master config file at '%s'", masterConfigFile))
	masterConfigLoaded = true
	masterConfig, masterConfigLoadError = configapilatest.ReadAndResolveMasterConfig(masterConfigFile)
	if masterConfigLoadError != nil {
		printMasterConfigLoadError(masterConfigFile, logger)
	} else {
		logger.Debug("DH0003", fmt.Sprintf("Found a master config file: %[1]s", masterConfigFile))
	}
	return masterConfig, masterConfigLoadError
}

func printMasterConfigLoadError(masterConfigFile string, logger *log.Logger) {
	logger.Error("DH0002", fmt.Sprintf("Could not read master config file '%s':\n(%T) %[2]v", masterConfigFile, masterConfigLoadError))
}
