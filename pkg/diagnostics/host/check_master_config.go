package host

import (
	"errors"
	"fmt"

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

	r.Debug("DH0001", fmt.Sprintf("Looking for master config file at '%s'", d.MasterConfigFile))
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(d.MasterConfigFile)
	if err != nil {
		r.Error("DH0002", err, fmt.Sprintf("Could not read master config file '%s':\n(%T) %[2]v", d.MasterConfigFile, err))
		return r
	}

	r.Info("DH0003", fmt.Sprintf("Found a master config file: %[1]s", d.MasterConfigFile))

	results := configvalidation.ValidateMasterConfig(masterConfig, nil)
	if len(results.Errors) > 0 {
		errText := fmt.Sprintf("Validation of master config file '%s' failed:\n", d.MasterConfigFile)
		for _, err := range results.Errors {
			errText += fmt.Sprintf("%v\n", err)
		}
		r.Error("DH0004", nil, errText)
	}
	if len(results.Warnings) > 0 {
		warnText := fmt.Sprintf("Validation of master config file '%s' warned:\n", d.MasterConfigFile)
		for _, warn := range results.Warnings {
			warnText += fmt.Sprintf("%v\n", warn)
		}
		r.Warn("DH0005", nil, warnText)
	}
	return r
}
