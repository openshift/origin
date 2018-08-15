package set

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
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
	  %[1]s deployment-hook dc/myapp --pre --volumes=data -- /var/lib/migrate-db.sh

	  # Set a mid deployment hook along with additional environment variables
	  %[1]s deployment-hook dc/myapp --mid --volumes=data -e VAR1=value1 -e VAR2=value2 -- /var/lib/prepare-deploy.sh`)
)

type DeploymentHookOptions struct {
	PrintFlags *genericclioptions.PrintFlags

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
	Client            dynamic.Interface
	Printer           printers.ResourcePrinter
	Builder           func() *resource.Builder
	Command           []string
	Resources         []string
	Environment       []string
	Volumes           []string
	Namespace         string
	ExplicitNamespace bool
	DryRun            bool
	FailurePolicy     appsv1.LifecycleHookFailurePolicy

	resource.FilenameOptions
	genericclioptions.IOStreams
}

func NewDeploymentHookOptions(streams genericclioptions.IOStreams) *DeploymentHookOptions {
	return &DeploymentHookOptions{
		PrintFlags:       genericclioptions.NewPrintFlags("hooks updated").WithTypeSetter(scheme.Scheme),
		IOStreams:        streams,
		FailurePolicyStr: "ignore",
	}
}

// NewCmdDeploymentHook implements the set deployment-hook command
func NewCmdDeploymentHook(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
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
	// TODO: remove shorthand 'v' in 3.12
	// this is done to trick pflag into allowing the duplicate registration.  The local value here wins
	cmd.Flags().StringSliceVarP(&o.Volumes, "v", "v", o.Volumes, "Volumes from the pod template to use in the deployment hook pod")
	cmd.Flags().MarkShorthandDeprecated("v", "Use --volumes instead.")
	cmd.Flags().StringSliceVar(&o.Volumes, "volumes", o.Volumes, "Volumes from the pod template to use in the deployment hook pod")
	cmd.Flags().StringVar(&o.FailurePolicyStr, "failure-policy", o.FailurePolicyStr, "The failure policy for the deployment hook. Valid values are: abort,retry,ignore")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set deployment hook will NOT contact api-server but run locally.")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *DeploymentHookOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.Resources = args
	if i := cmd.ArgsLenAtDash(); i != -1 {
		o.Resources = args[:i]
		o.Command = args[i:]
	}
	if len(o.Filenames) == 0 && len(args) < 1 {
		return kcmdutil.UsageErrorf(cmd, "one or more deployment configs must be specified as <name> or dc/<name>")
	}

	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}
	o.Builder = f.NewBuilder

	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	if len(o.FailurePolicyStr) > 0 {
		switch o.FailurePolicyStr {
		case "abort":
			o.FailurePolicy = appsv1.LifecycleHookFailurePolicyAbort
		case "ignore":
			o.FailurePolicy = appsv1.LifecycleHookFailurePolicyIgnore
		case "retry":
			o.FailurePolicy = appsv1.LifecycleHookFailurePolicyRetry
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
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
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

	patches := CalculatePatchesExternal(infos, func(info *resource.Info) (bool, error) {
		dc, ok := info.Object.(*appsv1.DeploymentConfig)
		if !ok {
			return false, nil
		}
		return o.updateDeploymentConfig(dc)
	})

	if singleItemImplied && len(patches) == 0 {
		name := infos[0].Name
		if infos[0].Mapping != nil {
			name = fmt.Sprintf("%s/%s", infos[0].Mapping.Resource.Resource, infos[0].Name)
		}
		return fmt.Errorf("%s is not a deployment config or does not have an applicable strategy", name)
	}

	allErrs := []error{}
	for _, patch := range patches {
		info := patch.Info
		name := getObjectName(info)
		if patch.Err != nil {
			allErrs = append(allErrs, fmt.Errorf("error: %s %v\n", name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			glog.V(1).Infof("info: %s was not changed\n", name)
			continue
		}

		if o.Local || o.DryRun {
			if err := o.Printer.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		actual, err := o.Client.Resource(info.Mapping.Resource).Namespace(info.Namespace).Patch(info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch deployment hook: %v\n", err))
			continue
		}

		if err := o.Printer.PrintObj(actual, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

func (o *DeploymentHookOptions) updateDeploymentConfig(dc *appsv1.DeploymentConfig) (bool, error) {
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

func (o *DeploymentHookOptions) updateRecreateParams(dc *appsv1.DeploymentConfig, strategyParams *appsv1.RecreateDeploymentStrategyParams) (bool, error) {
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

func (o *DeploymentHookOptions) updateRollingParams(dc *appsv1.DeploymentConfig, strategyParams *appsv1.RollingDeploymentStrategyParams) (bool, error) {
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

func (o *DeploymentHookOptions) lifecycleHook(dc *appsv1.DeploymentConfig) (*appsv1.LifecycleHook, error) {
	hook := &appsv1.LifecycleHook{
		FailurePolicy: o.FailurePolicy,
		ExecNewPod: &appsv1.ExecNewPodHook{
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
		// TODO Make external helpers
		env, _, err := utilenv.ParseEnv(o.Environment, nil)
		if err != nil {
			return nil, err
		}
		for i := range env {
			var versionedEnv corev1.EnvVar
			if err := legacyscheme.Scheme.Convert(&env[i], &versionedEnv, nil); err != nil {
				return nil, err
			}
			hook.ExecNewPod.Env = append(hook.ExecNewPod.Env, versionedEnv)
		}
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
