package rollout

import (
	"errors"
	"fmt"
	"io"

	"github.com/openshift/origin/pkg/oc/util/ocscheme"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"

	appsv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

var (
	rolloutLatestLong = templates.LongDesc(`
		Start a new rollout for a deployment config with the latest state from its triggers

		This command is appropriate for running manual rollouts. If you want full control over
		running new rollouts, use "oc set triggers --manual" to disable all triggers in your
		deployment config and then whenever you want to run a new deployment process, use this
		command in order to pick up the latest images found in the cluster that are pointed by
		your image change triggers.`)

	rolloutLatestExample = templates.Examples(`
	# Start a new rollout based on the latest images defined in the image change triggers.
	%[1]s rollout latest dc/nginx

	# Print the rolled out deployment config
	%[1]s rollout latest dc/nginx -o json`)
)

// RolloutLatestOptions holds all the options for the `rollout latest` command.
type RolloutLatestOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Builder   *resource.Builder
	Namespace string
	mapper    meta.RESTMapper
	Resource  string

	DryRun bool
	again  bool

	appsClient appsclient.DeploymentConfigsGetter
	kubeClient kclientset.Interface

	Printer printers.ResourcePrinter

	genericclioptions.IOStreams
}

func NewRolloutLatestOptions(streams genericclioptions.IOStreams) *RolloutLatestOptions {
	return &RolloutLatestOptions{
		IOStreams:  streams,
		PrintFlags: genericclioptions.NewPrintFlags("rolled out"),
	}
}

// NewCmdRolloutLatest implements the oc rollout latest subcommand.
func NewCmdRolloutLatest(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRolloutLatestOptions(streams)

	cmd := &cobra.Command{
		Use:     "latest DEPLOYMENTCONFIG",
		Short:   "Start a new rollout for a deployment config with the latest state from its triggers",
		Long:    rolloutLatestLong,
		Example: fmt.Sprintf(rolloutLatestExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.RunRolloutLatest())
		},
	}

	cmd.Flags().BoolVar(&o.again, "again", o.again, "If true, deploy the current pod template without updating state from triggers")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *RolloutLatestOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("one deployment config name is needed as argument.")
	}

	o.Resource = args[0]

	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")
	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}

	if o.PrintFlags.OutputFormat != nil && *o.PrintFlags.OutputFormat == "revision" {
		fmt.Fprintln(o.ErrOut, "--output=revision is deprecated. Use `--output=jsonpath={.status.latestVersion}` or `--output=go-template={{.status.latestVersion}}` instead")
		o.Printer = &revisionPrinter{}
	} else {
		o.Printer, err = o.PrintFlags.ToPrinter()
		if err != nil {
			return err
		}
	}

	o.kubeClient, err = f.ClientSet()
	if err != nil {
		return err
	}
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.appsClient, err = appsclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}

	o.Builder = f.NewBuilder()
	return nil
}

func (o *RolloutLatestOptions) RunRolloutLatest() error {
	infos, err := o.Builder.
		WithScheme(ocscheme.ReadingInternalScheme, ocscheme.ReadingInternalScheme.PrioritizedVersionsAllGroups()...).
		ContinueOnError().
		NamespaceParam(o.Namespace).
		ResourceNames("deploymentconfigs", o.Resource).
		SingleResourceType().
		Do().Infos()
	if err != nil {
		return err
	}

	if len(infos) != 1 {
		return errors.New("a deployment config name is required")
	}

	info := infos[0]
	config, ok := info.Object.(*appsv1.DeploymentConfig)
	if !ok {
		return fmt.Errorf("%s is not a deployment config", info.Name)
	}

	// TODO: Consider allowing one-off deployments for paused configs
	// See https://github.com/openshift/origin/issues/9903
	if config.Spec.Paused {
		return fmt.Errorf("cannot deploy a paused deployment config")
	}

	deploymentName := appsutil.LatestDeploymentNameForConfigAndVersion(config.Name, config.Status.LatestVersion)
	deployment, err := o.kubeClient.Core().ReplicationControllers(config.Namespace).Get(deploymentName, metav1.GetOptions{})
	switch {
	case err == nil:
		// Reject attempts to start a concurrent deployment.
		if !appsutil.IsTerminatedDeployment(deployment) {
			status := appsutil.DeploymentStatusFor(deployment)
			return fmt.Errorf("#%d is already in progress (%s).", config.Status.LatestVersion, status)
		}
	case !kerrors.IsNotFound(err):
		return err
	}

	dc := config
	if !o.DryRun {
		request := &appsv1.DeploymentRequest{
			Name:   config.Name,
			Latest: !o.again,
			Force:  true,
		}

		dc, err = o.appsClient.DeploymentConfigs(config.Namespace).Instantiate(config.Name, request)

		// Pre 1.4 servers don't support the instantiate endpoint. Fallback to incrementing
		// latestVersion on them.
		if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
			config.Status.LatestVersion++
			dc, err = o.appsClient.DeploymentConfigs(config.Namespace).Update(config)
		}

		if err != nil {
			return err
		}

		info.Refresh(dc, true)
	}

	return o.Printer.PrintObj(kcmdutil.AsDefaultVersionedOrOriginal(info.Object, info.Mapping), o.Out)
}

type revisionPrinter struct{}

func (p *revisionPrinter) PrintObj(obj runtime.Object, out io.Writer) error {
	dc, ok := obj.(*appsv1.DeploymentConfig)
	if !ok {
		return fmt.Errorf("%T is not a deployment config", obj)
	}

	fmt.Fprintf(out, fmt.Sprintf("%d", dc.Status.LatestVersion))
	return nil
}
