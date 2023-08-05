package summarize_audit_logs

import (
	"context"
	"os"

	"github.com/openshift/origin/pkg/invariants/auditloganalyzer"

	"k8s.io/client-go/kubernetes"

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
	restConfig, err := o.ConfigFlags.ToRESTConfig()
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	auditLogSummary, err := auditloganalyzer.GetKubeAuditLogSummary(ctx, kubeClient)
	if err != nil {
		return err
	}

	if err := auditloganalyzer.WriteAuditLogSummary(o.ArtifactDir, "", auditLogSummary); err != nil {
		return err
	}

	return nil
}
