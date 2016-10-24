package network

import (
	"fmt"

	kexec "k8s.io/kubernetes/pkg/util/exec"

	"github.com/openshift/origin/pkg/diagnostics/types"
)

const (
	CheckExternalNetworkName = "CheckExternalNetwork"
)

// CheckExternalNetwork is a Diagnostic to check accessibility of external network within a pod
type CheckExternalNetwork struct {
}

// Name is part of the Diagnostic interface and just returns name.
func (d CheckExternalNetwork) Name() string {
	return CheckExternalNetworkName
}

// Description is part of the Diagnostic interface and just returns the diagnostic description.
func (d CheckExternalNetwork) Description() string {
	return "Check that external network is accessible within a pod"
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d CheckExternalNetwork) CanRun() (bool, error) {
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d CheckExternalNetwork) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(CheckExternalNetworkName)

	externalAddress := "www.redhat.com"
	kexecer := kexec.New()
	if _, err := kexecer.Command("ping", "-c1", "-W2", externalAddress).CombinedOutput(); err != nil {
		// Admin may intentionally block access to the external network. If this check fails it doesn't necessarily mean that something is wrong. So just warn in this case.
		r.Warn("DExtNet1001", nil, fmt.Sprintf("Pinging external address %q failed. Check if the admin intentionally blocked access to the external network. Error: %s", externalAddress, err))
	}
	return r
}
