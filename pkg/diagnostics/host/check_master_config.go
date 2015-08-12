package host

import (
	"errors"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	configvalidation "github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// MasterConfigCheck is a Diagnostic to check that the master config file is valid
type MasterConfigCheck struct {
	MasterConfigFile string
}

const MasterConfigCheckName = "MasterConfigCheck"

func (d MasterConfigCheck) Name() string {
	return MasterConfigCheckName
}

func (d MasterConfigCheck) Description() string {
	return "Check the master config file"
}
func (d MasterConfigCheck) CanRun() (bool, error) {
	if len(d.MasterConfigFile) == 0 {
		return false, errors.New("must have master config file")
	}

	return true, nil
}
func (d MasterConfigCheck) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(MasterConfigCheckName)

	r.Debugf("discMCfile", "Looking for master config file at '%s'", d.MasterConfigFile)
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(d.MasterConfigFile)
	if err != nil {
		r.Errorf("discMCfail", err, "Could not read master config file '%s':\n(%T) %[2]v", d.MasterConfigFile, err)
		return r
	}

	r.Infof("discMCfound", "Found a master config file: %[1]s", d.MasterConfigFile)

	for _, err := range configvalidation.ValidateMasterConfig(masterConfig).Errors {
		r.Errorf("discMCinvalid", err, "Validation of master config file '%s' failed:\n(%T) %[2]v", d.MasterConfigFile, err)
	}
	return r
}
