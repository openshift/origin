package monitor_command

import (
	"github.com/openshift/origin/pkg/monitor/apiserveravailability"
	"github.com/openshift/origin/pkg/monitor/nodedetails"
	"github.com/spf13/cobra"
)

func NewMonitorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "monitor",
		SilenceErrors: true,
	}
	cmd.AddCommand(
		nodedetails.AuditLogSummaryCommand(),
		apiserveravailability.LogSummaryCommand(),
	)
	return cmd
}
