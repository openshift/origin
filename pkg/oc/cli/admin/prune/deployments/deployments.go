package deployments

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	appsv1 "github.com/openshift/api/apps/v1"
	appsv1client "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
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

	AppsClient appsv1client.DeploymentConfigsGetter
	KubeClient corev1client.CoreV1Interface

	genericclioptions.IOStreams
}

func NewPruneDeploymentsOptions(streams genericclioptions.IOStreams) *PruneDeploymentsOptions {
	return &PruneDeploymentsOptions{
		Confirm:         false,
		KeepYoungerThan: 60 * time.Minute,
		KeepComplete:    5,
		KeepFailed:      1,
		IOStreams:       streams,
	}
}

// NewCmdPruneDeployments implements the OpenShift cli prune deployments command.
func NewCmdPruneDeployments(f kcmdutil.Factory, parentName, name string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewPruneDeploymentsOptions(streams)
	cmd := &cobra.Command{
		Use:     name,
		Short:   "Remove old completed and failed deployments",
		Long:    deploymentsLongDesc,
		Example: fmt.Sprintf(deploymentsExample, parentName, name),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.Confirm, "confirm", o.Confirm, "If true, specify that deployment pruning should proceed. Defaults to false, displaying what would be deleted but not actually deleting anything.")
	cmd.Flags().BoolVar(&o.Orphans, "orphans", o.Orphans, "If true, prune all deployments where the associated DeploymentConfig no longer exists, the status is complete or failed, and the replica size is 0.")
	cmd.Flags().DurationVar(&o.KeepYoungerThan, "keep-younger-than", o.KeepYoungerThan, "Specify the minimum age of a deployment for it to be considered a candidate for pruning.")
	cmd.Flags().IntVar(&o.KeepComplete, "keep-complete", o.KeepComplete, "Per DeploymentConfig, specify the number of deployments whose status is complete that will be preserved whose replica size is 0.")
	cmd.Flags().IntVar(&o.KeepFailed, "keep-failed", o.KeepFailed, "Per DeploymentConfig, specify the number of deployments whose status is failed that will be preserved whose replica size is 0.")

	return cmd
}

// Complete turns a partially defined PruneDeploymentsOptions into a solvent structure
// which can be validated and used for pruning deployments.
func (o *PruneDeploymentsOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return kcmdutil.UsageErrorf(cmd, "no arguments are allowed to this command")
	}

	o.Namespace = metav1.NamespaceAll
	if cmd.Flags().Lookup("namespace").Changed {
		var err error
		o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return err
		}
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.KubeClient, err = corev1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.AppsClient, err = appsv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

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
	deploymentConfigs := []*appsv1.DeploymentConfig{}
	for i := range deploymentConfigList.Items {
		deploymentConfigs = append(deploymentConfigs, &deploymentConfigList.Items[i])
	}

	deploymentList, err := o.KubeClient.ReplicationControllers(o.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	deployments := []*corev1.ReplicationController{}
	for i := range deploymentList.Items {
		deployments = append(deployments, &deploymentList.Items[i])
	}

	options := PrunerOptions{
		KeepYoungerThan:   o.KeepYoungerThan,
		Orphans:           o.Orphans,
		KeepComplete:      o.KeepComplete,
		KeepFailed:        o.KeepFailed,
		DeploymentConfigs: deploymentConfigs,
		Deployments:       deployments,
	}
	pruner := NewPruner(options)

	w := tabwriter.NewWriter(o.Out, 10, 4, 3, ' ', 0)
	defer w.Flush()

	deploymentDeleter := &describingDeploymentDeleter{w: w}

	if o.Confirm {
		deploymentDeleter.delegate = NewDeploymentDeleter(o.KubeClient, o.KubeClient)
	} else {
		fmt.Fprintln(os.Stderr, "Dry run enabled - no modifications will be made. Add --confirm to remove deployments")
	}

	return pruner.Prune(deploymentDeleter)
}

// describingDeploymentDeleter prints information about each deployment it removes.
// If a delegate exists, its DeleteDeployment function is invoked prior to returning.
type describingDeploymentDeleter struct {
	w             io.Writer
	delegate      DeploymentDeleter
	headerPrinted bool
}

var _ DeploymentDeleter = &describingDeploymentDeleter{}

func (p *describingDeploymentDeleter) DeleteDeployment(deployment *corev1.ReplicationController) error {
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
