package monitor_command

import (
	summarize_audit_logs "github.com/openshift/origin/pkg/cmd/monitor_command/summarize-audit-logs"
	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/spf13/cobra"
)

func NewMonitorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "monitor",
		SilenceErrors: true,
	}
	cmd.AddCommand(
		summarize_audit_logs.AuditLogSummaryCommand(),
		apiserveravailability.LogSummaryCommand(),
	)
	return cmd
}
