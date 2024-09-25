package summarize_audit_logs

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"time"

	auditloganalyzer2 "github.com/openshift/origin/pkg/monitortests/kubeapiserver/auditloganalyzer"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/spf13/cobra"
)

type auditLogSummaryOptions struct {
	ArtifactDir string

	ConfigFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams
}

func AuditLogSummaryCommand() *cobra.Command {
	o := &auditLogSummaryOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}
	cmd := &cobra.Command{
		Use:   "summarize-audit-logs",
		Short: "Download and inspect audit logs for interesting things.",

		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.Run(context.Background())
		},
	}

	cmd.Flags().StringVar(&o.ArtifactDir, "artifact-dir", o.ArtifactDir, "The directory where monitor events will be stored.")
	o.ConfigFlags.AddFlags(cmd.Flags())
	return cmd
}

func (o auditLogSummaryOptions) Run(ctx context.Context) error {
	start, err := time.Parse(time.RFC3339, "2024-09-23T22:23:50Z")
	if err != nil {
		return err
	}
	installEnd, err := time.Parse(time.RFC3339, "2024-09-23T22:27:24Z")
	if err != nil {
		return err
	}
	requestCountTracking := auditloganalyzer2.CountsOverTime(metav1.Time{start})

	err = auditloganalyzer2.GetKubeAuditLogSummary(ctx, nil, nil, nil, []auditloganalyzer2.AuditEventHandler{requestCountTracking})
	if err != nil {
		return err
	}

	requestCountTracking.CountsForRun.TruncateDataAfterLastValue()
	installCounts := requestCountTracking.CountsForRun.SubsetDataAtTime(metav1.Time{installEnd})

	if err := requestCountTracking.CountsForRun.WriteContentToStorage("", "request-counts-for-all", "timeSuffix"); err != nil {
		return err
	}
	if err := installCounts.WriteContentToStorage("", "request-counts-for-install", "timeSuffix"); err != nil {
		return err
	}

	return nil
}
