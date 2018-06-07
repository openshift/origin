package set

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	oldresource "k8s.io/kubernetes/pkg/kubectl/resource"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oauth/generated/clientset/scheme"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	utilenv "github.com/openshift/origin/pkg/oc/util/env"
)

var (
	deploymentHookLong = templates.LongDesc(`
		Set or remove a deployment hook on a deployment config

		Deployment configs allow hooks to execute at different points in the lifecycle of the
		deployment, depending on the deployment strategy.

		For deployments with a Recreate strategy, a Pre, Mid, and Post hook can be specified.
		The Pre hook will execute before the deployment starts. The Mid hook will execute once the
		previous deployment has been scaled down to 0, but before the new one ramps up.
		The Post hook will execute once the deployment has completed.

		For deployments with a Rolling strategy, a Pre and Post hook can be specified.
		The Pre hook will execute before the deployment starts and the Post hook will execute once
		the deployment has completed.

		For each hook, a new pod will be started using one of the containers in the deployment's pod
		template with a specific command to execute. Additional environment variables may be specified
		for the hook, as well as which volumes from the pod template will be mounted on the hook pod.

		Each hook can have its own cancellation policy. One of: abort, retry, or ignore. Not all cancellation
		policies can be set on all hooks. For example, a Post hook on a rolling strategy does not support
		the abort policy, because at that point the deployment has already happened.`)

	deploymentHookExample = templates.Examples(`
		# Clear pre and post hooks on a deployment config
	  %[1]s deployment-hook dc/myapp --remove --pre --post

	  # Set the pre deployment hook to execute a db migration command for an application
	  # using the data volume from the application
	  %[1]s deployment-hook dc/myapp --pre -v data -- /var/lib/migrate-db.sh

	  # Set a mid deployment hook along with additional environment variables
	  %[1]s deployment-hook dc/myapp --mid -v data -e VAR1=value1 -e VAR2=value2 -- /var/lib/prepare-deploy.sh`)
)

type DeploymentHookOptions struct {
	PrintFlags *genericclioptions.PrintFlags
	oldresource.FilenameOptions
	genericclioptions.IOStreams

	Container        string
	Selector         string
	All              bool
	Local            bool
	Pre              bool
	Mid              bool
	Post             bool
	Remove           bool
	FailurePolicyStr string

	Mapper            meta.RESTMapper
	PrintObj          printers.ResourcePrinterFunc
	Builder           func() *oldresource.Builder
	Encoder           runtime.Encoder
	Command           []string
	Resources         []string
	Environment       []string
	Volumes           []string
	Namespace         string
	ExplicitNamespace bool
	DryRun            bool
	FailurePolicy     appsapi.LifecycleHookFailurePolicy
}

func NewDeploymentHookOptions(streams genericclioptions.IOStreams) *DeploymentHookOptions {
	return &DeploymentHookOptions{
		PrintFlags:       genericclioptions.NewPrintFlags("hooks updated").WithTypeSetter(scheme.Scheme),
		IOStreams:        streams,
		FailurePolicyStr: "ignore",
	}
}

// NewCmdDeploymentHook implements the set deployment-hook command
func NewCmdDeploymentHook(fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDeploymentHookOptions(streams)
	cmd := &cobra.Command{
		Use:     "deployment-hook DEPLOYMENTCONFIG --pre|--post|--mid -- CMD",
		Short:   "Update a deployment hook on a deployment config",
		Long:    deploymentHookLong,
		Example: fmt.Sprintf(deploymentHookExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	usage := "to use to edit the resource"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Container, "container", "c", o.Container, "The name of the container in the selected deployment config to use for the deployment hook")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter deployment configs")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, select all deployment configs in the namespace")
	cmd.Flags().BoolVar(&o.Remove, "remove", o.Remove, "If true, remove the specified deployment hook(s).")
	cmd.Flags().BoolVar(&o.Pre, "pre", o.Pre, "Set or remove a pre deployment hook")
	cmd.Flags().BoolVar(&o.Mid, "mid", o.Mid, "Set or remove a mid deployment hook")
	cmd.Flags().BoolVar(&o.Post, "post", o.Post, "Set or remove a post deployment hook")
	cmd.Flags().StringArrayVarP(&o.Environment, "environment", "e", o.Environment, "Environment variable to use in the deployment hook pod")
	cmd.Flags().StringSliceVarP(&o.Volumes, "volumes", "v", o.Volumes, "Volumes from the pod template to use in the deployment hook pod")
	cmd.Flags().MarkShorthandDeprecated("volumes", "Use --volumes instead.")
	cmd.Flags().StringVar(&o.FailurePolicyStr, "failure-policy", o.FailurePolicyStr, "The failure policy for the deployment hook. Valid values are: abort,retry,ignore")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set deployment hook will NOT contact api-server but run locally.")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *DeploymentHookOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	o.Resources = args
	if i := cmd.ArgsLenAtDash(); i != -1 {
		o.Resources = args[:i]
		o.Command = args[i:]
	}
	if len(o.Filenames) == 0 && len(args) < 1 {
		return kcmdutil.UsageErrorf(cmd, "one or more deployment configs must be specified as <name> or dc/<name>")
	}

	var err error
	o.Namespace, o.ExplicitNamespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	o.Mapper, _ = f.Object()
	o.Encoder = kcmdutil.InternalVersionJSONEncoder()
	o.Builder = f.NewBuilder

	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	o.PrintObj = printer.PrintObj

	if len(o.FailurePolicyStr) > 0 {
		switch o.FailurePolicyStr {
		case "abort":
			o.FailurePolicy = appsapi.LifecycleHookFailurePolicyAbort
		case "ignore":
			o.FailurePolicy = appsapi.LifecycleHookFailurePolicyIgnore
		case "retry":
			o.FailurePolicy = appsapi.LifecycleHookFailurePolicyRetry
		default:
			return kcmdutil.UsageErrorf(cmd, "valid values for --failure-policy are: abort, retry, ignore")
		}
	}

	return nil
}

func (o *DeploymentHookOptions) Validate() error {
	if o.Remove {
		if len(o.Command) > 0 ||
			len(o.Volumes) > 0 ||
			len(o.Environment) > 0 ||
			len(o.Container) > 0 {
			return fmt.Errorf("--remove may not be used with any option except --pre, --mid, or --post")
		}
		if !o.Pre && !o.Mid && !o.Post {
			return fmt.Errorf("you must specify at least one of --pre, --mid, or --post with the --remove flag")
		}
		return nil
	}

	cnt := 0
	if o.Pre {
		cnt++
	}
	if o.Mid {
		cnt++
	}
	if o.Post {
		cnt++
	}
	if cnt == 0 || cnt > 1 {
		return fmt.Errorf("you must specify one of --pre, --mid, or --post")
	}

	if len(o.Command) == 0 {
		return fmt.Errorf("you must specify a command for the deployment hook")
	}

	cmdutil.WarnAboutCommaSeparation(o.ErrOut, o.Environment, "--environment")

	return nil
}

func (o *DeploymentHookOptions) Run() error {
	b := o.Builder().
		Internal().
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		Flatten()

	if !o.Local {
		b = b.
			ResourceNames("deploymentconfigs", o.Resources...).
			LabelSelectorParam(o.Selector).
			Latest()
		if o.All {
			b = b.ResourceTypes("deploymentconfigs").SelectAllParam(o.All)
		}

	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	// FIXME-REBASE
	patches := CalculatePatches(infos, o.Encoder /*scheme.DefaultJSONEncoder()*/, func(info *oldresource.Info) (bool, error) {
		dc, ok := info.Object.(*appsapi.DeploymentConfig)
		if !ok {
			return false, nil
		}
		updated, err := o.updateDeploymentConfig(dc)
		return updated, err
	})

	if singleItemImplied && len(patches) == 0 {
		return fmt.Errorf("%s/%s is not a deployment config or does not have an applicable strategy", infos[0].Mapping.Resource, infos[0].Name)
	}

	allErrs := []error{}
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			allErrs = append(allErrs, fmt.Errorf("error: %s/%s %v\n", info.Mapping.Resource, info.Name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			continue
		}

		if o.Local || o.DryRun {
			if err := o.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		actual, err := oldresource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch deployment hook: %v\n", err))
			continue
		}

		if err := o.PrintObj(actual, o.Out); err != nil {
			// FIXME-REBASE
			// allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

func (o *DeploymentHookOptions) updateDeploymentConfig(dc *appsapi.DeploymentConfig) (bool, error) {
	var (
		err             error
		updatedRecreate bool
		updatedRolling  bool
	)

	if dc.Spec.Strategy.RecreateParams != nil {
		updatedRecreate, err = o.updateRecreateParams(dc, dc.Spec.Strategy.RecreateParams)
		if err != nil {
			return false, err
		}
	}
	if dc.Spec.Strategy.RollingParams != nil {
		updatedRolling, err = o.updateRollingParams(dc, dc.Spec.Strategy.RollingParams)
		if err != nil {
			return false, err
		}
	}
	return updatedRecreate || updatedRolling, nil
}

func (o *DeploymentHookOptions) updateRecreateParams(dc *appsapi.DeploymentConfig, strategyParams *appsapi.RecreateDeploymentStrategyParams) (bool, error) {
	var updated bool
	if o.Remove {
		if o.Pre && strategyParams.Pre != nil {
			updated = true
			strategyParams.Pre = nil
		}
		if o.Mid && strategyParams.Mid != nil {
			updated = true
			strategyParams.Mid = nil
		}
		if o.Post && strategyParams.Post != nil {
			updated = true
			strategyParams.Post = nil
		}
		return updated, nil
	}
	hook, err := o.lifecycleHook(dc)
	if err != nil {
		return true, err
	}
	switch {
	case o.Pre:
		strategyParams.Pre = hook
	case o.Mid:
		strategyParams.Mid = hook
	case o.Post:
		strategyParams.Post = hook
	}
	return true, nil
}

func (o *DeploymentHookOptions) updateRollingParams(dc *appsapi.DeploymentConfig, strategyParams *appsapi.RollingDeploymentStrategyParams) (bool, error) {
	var updated bool
	if o.Remove {
		if o.Pre && strategyParams.Pre != nil {
			updated = true
			strategyParams.Pre = nil
		}
		if o.Post && strategyParams.Post != nil {
			updated = true
			strategyParams.Post = nil
		}
		return updated, nil
	}
	hook, err := o.lifecycleHook(dc)
	if err != nil {
		return true, err
	}
	switch {
	case o.Pre:
		strategyParams.Pre = hook
	case o.Post:
		strategyParams.Post = hook
	}
	return true, nil
}

func (o *DeploymentHookOptions) lifecycleHook(dc *appsapi.DeploymentConfig) (*appsapi.LifecycleHook, error) {
	hook := &appsapi.LifecycleHook{
		FailurePolicy: o.FailurePolicy,
		ExecNewPod: &appsapi.ExecNewPodHook{
			Command: o.Command,
		},
	}
	if len(o.Container) > 0 {
		found := false
		for _, c := range dc.Spec.Template.Spec.Containers {
			if c.Name == o.Container {
				found = true
				break
			}
		}
		if !found {
			fmt.Fprintf(o.ErrOut, "warning: deployment config %q does not have a container named %q\n", dc.Name, o.Container)
		}
		hook.ExecNewPod.ContainerName = o.Container
	}
	if len(hook.ExecNewPod.ContainerName) == 0 {
		hook.ExecNewPod.ContainerName = dc.Spec.Template.Spec.Containers[0].Name
	}
	if len(o.Environment) > 0 {
		env, _, err := utilenv.ParseEnv(o.Environment, nil)
		if err != nil {
			return nil, err
		}
		hook.ExecNewPod.Env = env
	}
	if len(o.Volumes) > 0 {
		for _, v := range o.Volumes {
			found := false
			for _, podVolume := range dc.Spec.Template.Spec.Volumes {
				if podVolume.Name == v {
					found = true
					break
				}
			}
			if !found {
				fmt.Fprintf(o.ErrOut, "warning: deployment config %q does not have a volume named %q\n", dc.Name, v)
			}
		}
		hook.ExecNewPod.Volumes = o.Volumes
	}
	return hook, nil
}
