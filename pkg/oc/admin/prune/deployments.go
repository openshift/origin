package prune

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	"github.com/openshift/origin/pkg/oc/cli/deploymentconfigs/prune"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const PruneDeploymentsRecommendedName = "deployments"

var (
	deploymentsLongDesc = templates.LongDesc(`
		Prune old completed and failed deployments

		By default, the prune operation performs a dry run making no changes to the deployments.
		A --confirm flag is needed for changes to be effective.`)

	deploymentsExample = templates.Examples(`
		# Dry run deleting all but the last complete deployment for every deployment config
	  %[1]s %[2]s --keep-complete=1

	  # To actually perform the prune operation, the confirm flag must be appended
	  %[1]s %[2]s --keep-complete=1 --confirm`)
)

// PruneDeploymentsOptions holds all the required options for pruning deployments.
type PruneDeploymentsOptions struct {
	Confirm         bool
	Orphans         bool
	KeepYoungerThan time.Duration
	KeepComplete    int
	KeepFailed      int
	Namespace       string

	AppsClient appsclientinternal.DeploymentConfigsGetter
	KClient    kclientset.Interface
	Out        io.Writer
}

// NewCmdPruneDeployments implements the OpenShift cli prune deployments command.
func NewCmdPruneDeployments(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {
	opts := &PruneDeploymentsOptions{
		Confirm:         false,
		KeepYoungerThan: 60 * time.Minute,
		KeepComplete:    5,
		KeepFailed:      1,
	}

	cmd := &cobra.Command{
		Use:     name,
		Short:   "Remove old completed and failed deployments",
		Long:    deploymentsLongDesc,
		Example: fmt.Sprintf(deploymentsExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, args, out))
			kcmdutil.CheckErr(opts.Validate())
			kcmdutil.CheckErr(opts.Run())
		},
	}

	cmd.Flags().BoolVar(&opts.Confirm, "confirm", opts.Confirm, "If true, specify that deployment pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().BoolVar(&opts.Orphans, "orphans", opts.Orphans, "If true, prune all deployments where the associated DeploymentConfig no longer exists, the status is complete or failed, and the replica size is 0.")
	cmd.Flags().DurationVar(&opts.KeepYoungerThan, "keep-younger-than", opts.KeepYoungerThan, "Specify the minimum age of a deployment for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&opts.KeepComplete, "keep-complete", opts.KeepComplete, "Per DeploymentConfig, specify the number of deployments whose status is complete that will be preserved whose replica size is 0.")
	cmd.Flags().IntVar(&opts.KeepFailed, "keep-failed", opts.KeepFailed, "Per DeploymentConfig, specify the number of deployments whose status is failed that will be preserved whose replica size is 0.")

	return cmd
}

// Complete turns a partially defined PruneDeploymentsOptions into a solvent structure
// which can be validated and used for pruning deployments.
func (o *PruneDeploymentsOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) > 0 {
		return kcmdutil.UsageErrorf(cmd, "no arguments are allowed to this command")
	}

	o.Namespace = metav1.NamespaceAll
	if cmd.Flags().Lookup("namespace").Changed {
		var err error
		o.Namespace, _, err = f.DefaultNamespace()
		if err != nil {
			return err
		}
	}
	o.Out = out

	kClient, err := f.ClientSet()
	if err != nil {
		return err
	}
	appsClient, err := f.OpenshiftInternalAppsClient()
	if err != nil {
		return err
	}
	o.AppsClient = appsClient.Apps()
	o.KClient = kClient

	return nil
}

// Validate ensures that a PruneDeploymentsOptions is valid and can be used to execute pruning.
func (o PruneDeploymentsOptions) Validate() error {
	if o.KeepYoungerThan < 0 {
		return fmt.Errorf("--keep-younger-than must be greater than or equal to 0")
	}
	if o.KeepComplete < 0 {
		return fmt.Errorf("--keep-complete must be greater than or equal to 0")
	}
	if o.KeepFailed < 0 {
		return fmt.Errorf("--keep-failed must be greater than or equal to 0")
	}
	return nil
}

// Run contains all the necessary functionality for the OpenShift cli prune deployments command.
func (o PruneDeploymentsOptions) Run() error {
	deploymentConfigList, err := o.AppsClient.DeploymentConfigs(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	deploymentConfigs := []*appsapi.DeploymentConfig{}
	for i := range deploymentConfigList.Items {
		deploymentConfigs = append(deploymentConfigs, &deploymentConfigList.Items[i])
	}

	deploymentList, err := o.KClient.Core().ReplicationControllers(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	deployments := []*kapi.ReplicationController{}
	for i := range deploymentList.Items {
		deployments = append(deployments, &deploymentList.Items[i])
	}

	options := prune.PrunerOptions{
		KeepYoungerThan:   o.KeepYoungerThan,
		Orphans:           o.Orphans,
		KeepComplete:      o.KeepComplete,
		KeepFailed:        o.KeepFailed,
		DeploymentConfigs: deploymentConfigs,
		Deployments:       deployments,
	}
	pruner := prune.NewPruner(options)

	w := tabwriter.NewWriter(o.Out, 10, 4, 3, ' ', 0)
	defer w.Flush()

	deploymentDeleter := &describingDeploymentDeleter{w: w}

	if o.Confirm {
		deploymentDeleter.delegate = prune.NewDeploymentDeleter(o.KClient.Core(), o.KClient.Core())
	} else {
		fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove deployments")
	}

	return pruner.Prune(deploymentDeleter)
}

// describingDeploymentDeleter prints information about each deployment it removes.
// If a delegate exists, its DeleteDeployment function is invoked prior to returning.
type describingDeploymentDeleter struct {
	w             io.Writer
	delegate      prune.DeploymentDeleter
	headerPrinted bool
}

var _ prune.DeploymentDeleter = &describingDeploymentDeleter{}

func (p *describingDeploymentDeleter) DeleteDeployment(deployment *kapi.ReplicationController) error {
	if !p.headerPrinted {
		p.headerPrinted = true
		fmt.Fprintln(p.w, "NAMESPACE\tNAME")
	}

	fmt.Fprintf(p.w, "%s\t%s\n", deployment.Namespace, deployment.Name)

	if p.delegate == nil {
		return nil
	}

	if err := p.delegate.DeleteDeployment(deployment); err != nil {
		return err
	}

	return nil
}
