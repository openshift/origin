package host

import (
	"errors"
	"fmt"

	master "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configvalidation "github.com/openshift/origin/pkg/cmd/server/apis/config/validation"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
)

// MasterConfigCheck is a Diagnostic to check that the master config file is valid
type MasterConfigCheck struct {
	MasterConfigFile string
	masterConfig     *master.MasterConfig
}

const MasterConfigCheckName = "MasterConfigCheck"

func (d *MasterConfigCheck) Name() string {
	return MasterConfigCheckName
}

func (d *MasterConfigCheck) Description() string {
	return "Check the master config file"
}

func (d *MasterConfigCheck) Requirements() (client bool, host bool) {
	return false, true
}

func (d *MasterConfigCheck) Complete(logger *log.Logger) error {
	masterConfig, err := GetMasterConfig(d.MasterConfigFile, logger)
	if err != nil {
		return err
	}
	d.masterConfig = masterConfig
	return nil
}

func (d *MasterConfigCheck) CanRun() (bool, error) {
	if len(d.MasterConfigFile) == 0 {
		return false, errors.New("No master config file was detected")
	}

	return true, nil
}

func (d *MasterConfigCheck) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(MasterConfigCheckName)

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
