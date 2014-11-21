package osc

import (
	kctl "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/openshift/origin/pkg/cmd/kubectl"
)

// TODO Printer behaves exactly like kubectl for now. Moving forward
// we will want a strategy around UX that better fits end-users.
func NewHumanReadablePrinter(noHeaders bool) *kctl.HumanReadablePrinter {
	return kubectl.NewHumanReadablePrinter(noHeaders)
}
