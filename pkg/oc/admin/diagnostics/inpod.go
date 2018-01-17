package diagnostics

import (
	"io"

	"github.com/spf13/cobra"
)

const (
	DiagnosticsInPodRecommendedName = "openshift-diagnostics"
)

// NewCmdDiagnosticsInPod defines the base command that is used inside of a pod
// orchestrated by a regular diagnostic, to report from the in-pod vantage point.
func NewCmdDiagnosticsInPod(name string, fullName string, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Diagnose OpenShift from inside a pod",
	}
	cmd.AddCommand(
		NewCommandPodDiagnostics("diagnostic-pod", out),
		NewCommandNetworkPodDiagnostics("network-diagnostic-pod", out),
	)

	return cmd
}
