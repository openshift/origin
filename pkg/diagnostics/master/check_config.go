package master

import (
	"errors"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	configvalidation "github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/diagnostics/log"
)

// MasterConfigCheck
type MasterConfigCheck struct {
	MasterConfigFile string

	Log *log.Logger
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
func (d MasterConfigCheck) Check() (bool, []log.Message, []error, []error) {
	if _, err := d.CanRun(); err != nil {
		return false, nil, nil, []error{err}
	}

	d.Log.Debugf("discMCfile", "Looking for master config file at '%s'", d.MasterConfigFile)
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(d.MasterConfigFile)
	if err != nil {
		d.Log.Errorf("discMCfail", "Could not read master config file '%s':\n(%T) %[2]v", d.MasterConfigFile, err)

		return false, nil, nil, []error{err}
	}

	d.Log.Infof("discMCfound", "Found a master config file:\n%[1]s", d.MasterConfigFile)

	if validationResults := configvalidation.ValidateMasterConfig(masterConfig); len(validationResults.Errors) > 0 {
		return false, nil, nil, validationResults.Errors
	}

	return true, nil, nil, nil
}
