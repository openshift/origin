package apiserveravailability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type auditLogSummaryOptions struct {
	ArtifactDir string

	ConfigFlags *genericclioptions.ConfigFlags
	IOStreams   genericclioptions.IOStreams
}

func LogSummaryCommand() *cobra.Command {
	o := &auditLogSummaryOptions{
		ConfigFlags: genericclioptions.NewConfigFlags(true),
		IOStreams: genericclioptions.IOStreams{
			In:     os.Stdin,
			Out:    os.Stdout,
			ErrOut: os.Stderr,
		},
	}
	cmd := &cobra.Command{
		Use:   "summarize-cluster-logs",
		Short: "Download and inspect some logs for interesting things.",

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

	summary, err := SummarizeInterestingPodLogs(ctx, kubeClient)
	if err != nil {
		return err
	}

	if err := WriteAPIServerAccessClientFailureSummary(o.ArtifactDir, "", summary); err != nil {
		return err
	}

	return nil
}

func WriteAPIServerAccessClientFailureSummary(artifactDir, timeSuffix string, summary *APIServerClientAccessFailureSummary) error {
	summaryBytes, err := json.MarshalIndent(summary, "", "    ")
	if err != nil {
		return err
	}
	summaryPath := filepath.Join(artifactDir, fmt.Sprintf("apiserver-client-access-failure-summary_%s.json", timeSuffix))
	if err := os.WriteFile(summaryPath, summaryBytes, 0644); err != nil {
		return fmt.Errorf("failed to write %v: %w", summaryPath, err)
	}
	return nil
}
