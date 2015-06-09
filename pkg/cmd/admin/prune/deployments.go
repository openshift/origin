package prune

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/prune"
)

const deploymentsLongDesc = `%s %s - Remove older completed and failed deployments`

const PruneDeploymentsRecommendedName = "deployments"

type pruneDeploymentConfig struct {
	Confirm         bool
	KeepYoungerThan time.Duration
	Orphans         bool
	KeepComplete    int
	KeepFailed      int
}

func NewCmdPruneDeployments(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	cfg := &pruneDeploymentConfig{
		Confirm:         false,
		KeepYoungerThan: 60 * time.Minute,
		KeepComplete:    5,
		KeepFailed:      1,
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Remove completed and failed deployments",
		Long:  fmt.Sprintf(deploymentsLongDesc, parentName, name),

		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				glog.Fatalf("No arguments are allowed to this command")
			}

			osClient, kclient, err := f.Clients()
			if err != nil {
				cmdutil.CheckErr(err)
			}

			deploymentConfigList, err := osClient.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			if err != nil {
				cmdutil.CheckErr(err)
			}

			deploymentList, err := kclient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
			if err != nil {
				cmdutil.CheckErr(err)
			}

			deploymentConfigs := []*deployapi.DeploymentConfig{}
			for i := range deploymentConfigList.Items {
				deploymentConfigs = append(deploymentConfigs, &deploymentConfigList.Items[i])
			}

			deployments := []*kapi.ReplicationController{}
			for i := range deploymentList.Items {
				deployments = append(deployments, &deploymentList.Items[i])
			}

			var deploymentPruneFunc prune.PruneFunc

			w := tabwriter.NewWriter(out, 10, 4, 3, ' ', 0)
			defer w.Flush()

			describingPruneDeploymentFunc := func(deployment *kapi.ReplicationController) error {
				fmt.Fprintf(w, "%s\t%s\n", deployment.Namespace, deployment.Name)
				return nil
			}

			switch cfg.Confirm {
			case true:
				deploymentPruneFunc = func(deployment *kapi.ReplicationController) error {
					describingPruneDeploymentFunc(deployment)
					err := kclient.ReplicationControllers(deployment.Namespace).Delete(deployment.Name)
					if err != nil {
						return err
					}
					return nil
				}
			default:
				fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made.")
				deploymentPruneFunc = describingPruneDeploymentFunc
			}

			fmt.Fprintln(w, "NAMESPACE\tNAME")
			pruneTask := prune.NewPruneTasker(deploymentConfigs, deployments, cfg.KeepYoungerThan, cfg.Orphans, cfg.KeepComplete, cfg.KeepFailed, deploymentPruneFunc)
			err = pruneTask.PruneTask()
			if err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&cfg.Confirm, "confirm", cfg.Confirm, "Specify that deployment pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().BoolVar(&cfg.Orphans, "orphans", cfg.Orphans, "Prune all deployments where the associated DeploymentConfig no longer exists, the status is complete or failed, and the replica size is 0.")
	cmd.Flags().DurationVar(&cfg.KeepYoungerThan, "keep-younger-than", cfg.KeepYoungerThan, "Specify the minimum age of a deployment for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&cfg.KeepComplete, "keep-complete", cfg.KeepComplete, "Per DeploymentConfig, specify the number of deployments whose status is complete that will be preserved whose replica size is 0.")
	cmd.Flags().IntVar(&cfg.KeepFailed, "keep-failed", cfg.KeepFailed, "Per DeploymentConfig, specify the number of deployments whose status is failed that will be preserved whose replica size is 0.")

	return cmd
}
