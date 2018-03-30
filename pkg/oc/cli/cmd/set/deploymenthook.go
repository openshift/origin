package set

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
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
	Out io.Writer
	Err io.Writer

	Builder *resource.Builder
	Infos   []*resource.Info

	Encoder runtime.Encoder

	Filenames []string
	Container string
	Selector  string
	All       bool

	Output string

	ShortOutput bool
	Local       bool
	Mapper      meta.RESTMapper

	PrintObject func([]*resource.Info) error

	Pre    bool
	Mid    bool
	Post   bool
	Remove bool

	Cmd *cobra.Command

	Command     []string
	Environment []string
	Volumes     []string

	FailurePolicy appsapi.LifecycleHookFailurePolicy
}

// NewCmdDeploymentHook implements the set deployment-hook command
func NewCmdDeploymentHook(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &DeploymentHookOptions{
		Out: out,
		Err: errOut,
	}
	cmd := &cobra.Command{
		Use:     "deployment-hook DEPLOYMENTCONFIG --pre|--post|--mid -- CMD",
		Short:   "Update a deployment hook on a deployment config",
		Long:    deploymentHookLong,
		Example: fmt.Sprintf(deploymentHookExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			if err := options.Run(); err != nil {
				// TODO: move me to kcmdutil
				if err == kcmdutil.ErrExit {
					os.Exit(1)
				}
				kcmdutil.CheckErr(err)
			}
		},
	}

	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().StringVarP(&options.Container, "container", "c", options.Container, "The name of the container in the selected deployment config to use for the deployment hook")
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter deployment configs")
	cmd.Flags().BoolVar(&options.All, "all", options.All, "If true, select all deployment configs in the namespace")
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")

	cmd.Flags().BoolVar(&options.Remove, "remove", options.Remove, "If true, remove the specified deployment hook(s).")
	cmd.Flags().BoolVar(&options.Pre, "pre", options.Pre, "Set or remove a pre deployment hook")
	cmd.Flags().BoolVar(&options.Mid, "mid", options.Mid, "Set or remove a mid deployment hook")
	cmd.Flags().BoolVar(&options.Post, "post", options.Post, "Set or remove a post deployment hook")

	cmd.Flags().StringArrayVarP(&options.Environment, "environment", "e", options.Environment, "Environment variable to use in the deployment hook pod")
	cmd.Flags().StringSliceVarP(&options.Volumes, "volumes", "v", options.Volumes, "Volumes from the pod template to use in the deployment hook pod")
	cmd.Flags().MarkShorthandDeprecated("volumes", "Use --volumes instead.")

	cmd.Flags().String("failure-policy", "ignore", "The failure policy for the deployment hook. Valid values are: abort,retry,ignore")

	cmd.Flags().BoolVar(&options.Local, "local", false, "If true, set deployment hook will NOT contact api-server but run locally.")

	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *DeploymentHookOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	resources := args
	if i := cmd.ArgsLenAtDash(); i != -1 {
		resources = args[:i]
		o.Command = args[i:]
	}
	if len(o.Filenames) == 0 && len(args) < 1 {
		return kcmdutil.UsageErrorf(cmd, "one or more deployment configs must be specified as <name> or dc/<name>")
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Cmd = cmd

	mapper, _ := f.Object()
	o.Builder = f.NewBuilder().
		Internal().
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		Flatten()
	if !o.Local {
		o.Builder = o.Builder.
			ResourceNames("deploymentconfigs", resources...).
			LabelSelectorParam(o.Selector).
			Latest()
		if o.All {
			o.Builder.ResourceTypes("deploymentconfigs").SelectAllParam(o.All)
		}

	}

	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.PrintObject = func(infos []*resource.Info) error {
		return f.PrintResourceInfos(cmd, o.Local, infos, o.Out)
	}

	o.Encoder = f.JSONEncoder()
	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"
	o.Mapper = mapper

	failurePolicyString := kcmdutil.GetFlagString(cmd, "failure-policy")
	if len(failurePolicyString) > 0 {
		switch failurePolicyString {
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

	cmdutil.WarnAboutCommaSeparation(o.Err, o.Environment, "--environment")

	return nil
}

func (o *DeploymentHookOptions) Run() error {
	infos := o.Infos
	singleItemImplied := len(o.Infos) <= 1
	if o.Builder != nil {
		loaded, err := o.Builder.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
		if err != nil {
			return err
		}
		infos = loaded
	}

	patches := CalculatePatches(infos, o.Encoder, func(info *resource.Info) (bool, error) {
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

	if len(o.Output) > 0 || o.Local || kcmdutil.GetDryRunFlag(o.Cmd) {
		return o.PrintObject(infos)
	}

	failed := false
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			fmt.Fprintf(o.Err, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, patch.Err)
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			fmt.Fprintf(o.Err, "info: %s %q was not changed\n", info.Mapping.Resource, info.Name)
			continue
		}

		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			fmt.Fprintf(o.Err, "error: %v\n", err)
			failed = true
			continue
		}

		info.Refresh(obj, true)
		kcmdutil.PrintSuccess(o.Mapper, o.ShortOutput, o.Out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if failed {
		return kcmdutil.ErrExit
	}
	return nil
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
			fmt.Fprintf(o.Err, "warning: deployment config %q does not have a container named %q\n", dc.Name, o.Container)
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
				fmt.Fprintf(o.Err, "warning: deployment config %q does not have a volume named %q\n", dc.Name, v)
			}
		}
		hook.ExecNewPod.Volumes = o.Volumes
	}
	return hook, nil
}
