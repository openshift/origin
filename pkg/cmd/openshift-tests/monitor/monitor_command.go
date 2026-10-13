package monitor

import (
	"github.com/openshift/origin/pkg/cmd/openshift-tests/monitor/run"
	summarize_audit_logs "github.com/openshift/origin/pkg/cmd/openshift-tests/monitor/summarize-audit-logs"
	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewMonitorCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "monitor",
		SilenceErrors: true,
	}
	cmd.AddCommand(
		run.NewRunCommand(streams),
		summarize_audit_logs.AuditLogSummaryCommand(),
		apiserveravailability.LogSummaryCommand(),
	)
	return cmd
}
