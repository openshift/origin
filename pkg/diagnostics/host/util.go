package host

import (
	"fmt"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// this would be bad practice if there were ever a need to load more than one master config for diagnostics.
// however we will proceed with the assumption that will never be necessary.
var (
	masterConfigLoaded    = false
	masterConfig          *configapi.MasterConfig
	masterConfigLoadError error
)

func GetMasterConfig(r types.DiagnosticResult, masterConfigFile string) (*configapi.MasterConfig, error) {
	if masterConfigLoaded { // no need to do this more than once
		return masterConfig, masterConfigLoadError
	}
	r.Debug("DH0001", fmt.Sprintf("Looking for master config file at '%s'", masterConfigFile))
	masterConfigLoaded = true
	masterConfig, masterConfigLoadError = configapilatest.ReadAndResolveMasterConfig(masterConfigFile)
	if masterConfigLoadError != nil {
		r.Error("DH0002", masterConfigLoadError, fmt.Sprintf("Could not read master config file '%s':\n(%T) %[2]v", masterConfigFile, masterConfigLoadError))
	} else {
		r.Debug("DH0003", fmt.Sprintf("Found a master config file: %[1]s", masterConfigFile))
	}
	return masterConfig, masterConfigLoadError
}
