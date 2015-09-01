package host

import (
	"errors"
	"fmt"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	configvalidation "github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// NodeConfigCheck is a Diagnostic to check that the node config file is valid
type NodeConfigCheck struct {
	NodeConfigFile string
}

const NodeConfigCheckName = "NodeConfigCheck"

func (d NodeConfigCheck) Name() string {
	return NodeConfigCheckName
}

func (d NodeConfigCheck) Description() string {
	return "Check the node config file"
}
func (d NodeConfigCheck) CanRun() (bool, error) {
	if len(d.NodeConfigFile) == 0 {
		return false, errors.New("must have node config file")
	}

	return true, nil
}
func (d NodeConfigCheck) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(NodeConfigCheckName)
	r.Debug("DH1001", fmt.Sprintf("Looking for node config file at '%s'", d.NodeConfigFile))
	nodeConfig, err := configapilatest.ReadAndResolveNodeConfig(d.NodeConfigFile)
	if err != nil {
		r.Error("DH1002", err, fmt.Sprintf("Could not read node config file '%s':\n(%T) %[2]v", d.NodeConfigFile, err))
		return r
	}

	r.Info("DH1003", fmt.Sprintf("Found a node config file: %[1]s", d.NodeConfigFile))

	for _, err := range configvalidation.ValidateNodeConfig(nodeConfig) {
		r.Error("DH1004", err, fmt.Sprintf("Validation of node config file '%s' failed:\n(%T) %[2]v", d.NodeConfigFile, err))
	}
	return r
}
