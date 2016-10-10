package rollout

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

const (
	rolloutLatestLong = `
Start a new rollout for a deployment config with the latest state from its triggers

This command is appropriate for running manual rollouts. If you want full control over
running new rollouts, use "oc set triggers --manual" to disable all triggers in your
deployment config and then whenever you want to run a new deployment process, use this
command in order to pick up the latest images found in the cluster that are pointed by
your image change triggers.`

	rolloutLatestExample = `  # Start a new rollout based on the latest images defined in the image change triggers.
  %[1]s rollout latest dc/nginx
`
)

// RolloutLatestOptions holds all the options for the `rollout latest` command.
// TODO: Support --dry-run
type RolloutLatestOptions struct {
	mapper meta.RESTMapper
	typer  runtime.ObjectTyper
	infos  []*resource.Info

	out         io.Writer
	shortOutput bool

	oc              client.Interface
	kc              kclient.Interface
	baseCommandName string
}

// NewCmdRolloutLatest implements the oc rollout latest subcommand.
func NewCmdRolloutLatest(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &RolloutLatestOptions{
		baseCommandName: fullName,
	}

	cmd := &cobra.Command{
		Use:     "latest DEPLOYMENTCONFIG",
		Short:   "Start a new rollout for a deployment config with the latest state from its triggers",
		Long:    rolloutLatestLong,
		Example: fmt.Sprintf(rolloutLatestExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := opts.Complete(f, cmd, args, out)
			kcmdutil.CheckErr(err)

			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			err = opts.RunRolloutLatest()
			kcmdutil.CheckErr(err)
		},
	}

	kcmdutil.AddOutputFlagsForMutation(cmd)

	return cmd
}

func (o *RolloutLatestOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("one deployment config name is needed as argument.")
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.oc, o.kc, err = f.Clients()
	if err != nil {
		return err
	}

	o.mapper, o.typer = f.Object(false)
	o.infos, err = resource.NewBuilder(o.mapper, o.typer, resource.ClientMapperFunc(f.ClientForMapping), f.Decoder(true)).
		ContinueOnError().
		NamespaceParam(namespace).
		ResourceNames("deploymentconfigs", args[0]).
		SingleResourceType().
		Do().Infos()
	if err != nil {
		return err
	}

	o.out = out
	o.shortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"

	return nil
}

func (o RolloutLatestOptions) Validate() error {
	if len(o.infos) != 1 {
		return errors.New("a deployment config name is required.")
	}
	return nil
}

func (o RolloutLatestOptions) RunRolloutLatest() error {
	info := o.infos[0]
	config, ok := info.Object.(*deployapi.DeploymentConfig)
	if !ok {
		return fmt.Errorf("%s is not a deployment config", info.Name)
	}

	// TODO: Consider allowing one-off deployments for paused configs
	// See https://github.com/openshift/origin/issues/9903
	if config.Spec.Paused {
		return fmt.Errorf("cannot deploy a paused deployment config")
	}

	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := o.kc.ReplicationControllers(config.Namespace).Get(deploymentName)
	switch {
	case err == nil:
		// Reject attempts to start a concurrent deployment.
		if !deployutil.IsTerminatedDeployment(deployment) {
			status := deployutil.DeploymentStatusFor(deployment)
			return fmt.Errorf("#%d is already in progress (%s).", config.Status.LatestVersion, status)
		}
	case !kerrors.IsNotFound(err):
		return err
	}

	request := &deployapi.DeploymentRequest{
		Name:   config.Name,
		Latest: true,
		Force:  true,
	}

	dc, err := o.oc.DeploymentConfigs(config.Namespace).Instantiate(request)
	// Pre 1.4 servers don't support the instantiate endpoint. Fallback to incrementing
	// latestVersion on them.
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		config.Status.LatestVersion++
		dc, err = o.oc.DeploymentConfigs(config.Namespace).Update(config)
	}
	if err != nil {
		return err
	}

	info.Refresh(dc, true)

	kcmdutil.PrintSuccess(o.mapper, o.shortOutput, o.out, info.Mapping.Resource, info.Name, "rolled out")
	return nil
}
