package set

import (
	"fmt"
	"io"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	deploymentStrategyLong = templates.LongDesc(`
		Set and manage the deployment strategy for a deployment config and deployment
		and update strategy for statefulset and daemonset.`)

	deploymentStrategyExample = templates.Examples(`
		# Switch the deployment strategy for dc/myapp to recreate
	  %[1]s deployment-strategy dc/myapp --recreate

		# Change the maxSurge for deployment config
	  %[1]s deployment-strategy dc/myapp --rolling --max-surge=2

		# Change the update strategy to OnDelete for a statefulset and also set the partiion
	  %[1]s deployment-strategy ss/myapp --rolling --partition=10`)
)

// DeploymentStrategyOptions contain options for set deployment-strategy.
type DeploymentStrategyOptions struct {
	Out         io.Writer
	Err         io.Writer
	Builder     *resource.Builder
	Infos       []*resource.Info
	Encoder     runtime.Encoder
	Filenames   []string
	Output      string
	Mapper      meta.RESTMapper
	Selector    string
	All         bool
	ShortOutput bool
	Local       bool

	PrintObject func([]*resource.Info) error
	Cmd         *cobra.Command

	// DeploymentConfig and Deployment
	Recreate bool
	Rolling  bool
	Custom   bool

	// StatefulSet and DaemonSet
	OnDelete bool

	// DeploymentConfig strategies options
	TimeoutSeconds      int64
	UpdatePeriodSeconds int64
	IntervalSeconds     int64
	MaxUnavailable      string
	MaxSurge            string
	Image               string
	Command             []string
	Environment         []string

	// StatefulSet
	Partition int32
}

// NewCmdDeploymentStrategy implements the set deployment-strategy command
func NewCmdDeploymentStrategy(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &DeploymentStrategyOptions{
		Out: out,
		Err: errOut,
	}
	cmd := &cobra.Command{
		Use:     "deployment-strategy RESOURCE --recreate|--rolling|--on-delete|--custom",
		Short:   "Update a deployment strategy",
		Long:    deploymentStrategyLong,
		Example: fmt.Sprintf(deploymentStrategyExample, fullName),
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

	cmd.Flags().Int64Var(&options.TimeoutSeconds, "timeout-seconds", deployapi.DefaultRecreateTimeoutSeconds, "The time to wait for updates before giving up.")
	cmd.Flags().Int64Var(&options.IntervalSeconds, "interval-seconds", deployapi.DefaultRollingIntervalSeconds, "The time to wait between polling deployment status after update in rolling strategy.")
	cmd.Flags().Int64Var(&options.UpdatePeriodSeconds, "update-period-seconds", deployapi.DefaultRollingUpdatePeriodSeconds, "The time to wait between individual pod updates in rolling strategy.")
	cmd.Flags().Int32Var(&options.Partition, "partition", 0, "A number that indicates the ordinal at which the StatefulSet should be partitioned.")
	cmd.Flags().StringVar(&options.MaxUnavailable, "max-unavailable", "", "The maximum number of pods that can be unavailable during the update in rolling strategy.")
	cmd.Flags().StringVar(&options.MaxSurge, "max-surge", "", "The maximum number of pods that can be scheduled above the original number of pods in rolling strategy.")
	cmd.Flags().StringVar(&options.Image, "image", "", "An image which can carry out a custom deployment strategy.")
	cmd.Flags().StringArrayVarP(&options.Environment, "environment", "e", options.Environment, "Environment variables to use in the custom deployment pod")

	cmd.Flags().BoolVar(&options.Recreate, "recreate", options.Recreate, "Set a Recreate strategy")
	cmd.Flags().BoolVar(&options.Rolling, "rolling", options.Rolling, "Set a Rolling strategy")
	cmd.Flags().BoolVar(&options.Custom, "custom", options.Custom, "Set a Custom strategy")
	cmd.Flags().BoolVar(&options.OnDelete, "on-delete", options.OnDelete, "Set a OnDelete strategy")

	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")
	cmd.Flags().BoolVar(&options.Local, "local", false, "If true, set strategy will NOT contact api-server but run locally.")
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter resources")
	cmd.Flags().BoolVar(&options.All, "all", options.All, "If true, select all resources in the namespace of the specified resource types")

	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *DeploymentStrategyOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	resources := args
	if i := cmd.ArgsLenAtDash(); i != -1 {
		resources = args[:i]
		o.Command = args[i:]
	}
	if len(o.Filenames) == 0 && len(args) < 1 {
		return kcmdutil.UsageErrorf(cmd, "one or more resources must be specified as <name> or dc/<name>")
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Cmd = cmd

	mapper, _ := f.Object()
	o.Builder = f.NewBuilder(!o.Local).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		Flatten()

	if !o.Local {
		o.Builder = o.Builder.
			SelectorParam(o.Selector).
			ResourceTypeOrNameArgs(o.All, resources...)
	}

	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.PrintObject = func(infos []*resource.Info) error {
		return f.PrintResourceInfos(cmd, o.Local, infos, o.Out)
	}

	o.Encoder = f.JSONEncoder()
	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"
	o.Mapper = mapper

	return nil
}

// Validate validates the options for different strategies.
func (o *DeploymentStrategyOptions) Validate() error {
	switch {
	case o.Recreate:
		return o.validateRecreate()
	case o.Rolling:
		return o.validateRolling()
	case o.OnDelete:
		return o.validateOnDelete()
	case o.Custom:
		return o.validateCustom()
	default:
		return kcmdutil.UsageErrorf(o.Cmd, "strategy must be selected")
	}
}

// TODO: Add validation of params here? It might be overlapping with API validation.
func (o *DeploymentStrategyOptions) validateRecreate() error {
	if o.Cmd.Flags().Changed("max-surge") {
		return kcmdutil.UsageErrorf(o.Cmd, "max-surge can be used only with rolling strategy")
	}
	if o.Cmd.Flags().Changed("max-unavailable") {
		return kcmdutil.UsageErrorf(o.Cmd, "max-unavailable can be used only with rolling strategy")
	}
	if o.Cmd.Flags().Changed("partition") {
		return kcmdutil.UsageErrorf(o.Cmd, "partition can be used only with rolling update strategy")
	}
	if o.Cmd.Flags().Changed("image") || o.Cmd.Flags().Changed("environment") {
		return kcmdutil.UsageErrorf(o.Cmd, "image and environment can be used only with custom strategy")
	}
	return nil
}

func (o *DeploymentStrategyOptions) validateRolling() error {
	if o.Cmd.Flags().Changed("image") || o.Cmd.Flags().Changed("environment") {
		return kcmdutil.UsageErrorf(o.Cmd, "image and environment can be used only with custom strategy")
	}
	return nil
}

func (o *DeploymentStrategyOptions) validateOnDelete() error {
	if o.Cmd.Flags().Changed("max-surge") {
		return kcmdutil.UsageErrorf(o.Cmd, "max-surge can be used only with rolling strategy")
	}
	if o.Cmd.Flags().Changed("max-unavailable") {
		return kcmdutil.UsageErrorf(o.Cmd, "max-unavailable can be used only with rolling strategy")
	}
	if o.Cmd.Flags().Changed("image") || o.Cmd.Flags().Changed("environment") {
		return kcmdutil.UsageErrorf(o.Cmd, "image and environment can be used only with custom strategy")
	}
	if o.Cmd.Flags().Changed("partition") {
		return kcmdutil.UsageErrorf(o.Cmd, "partition can be used only with rolling update strategy")
	}
	return nil
}

func (o *DeploymentStrategyOptions) validateCustom() error {
	if o.Cmd.Flags().Changed("max-surge") {
		return kcmdutil.UsageErrorf(o.Cmd, "max-surge can be used only with rolling strategy")
	}
	if o.Cmd.Flags().Changed("max-unavailable") {
		return kcmdutil.UsageErrorf(o.Cmd, "max-unavailable can be used only with rolling strategy")
	}
	if o.Cmd.Flags().Changed("partition") {
		return kcmdutil.UsageErrorf(o.Cmd, "partition can be used only with rolling update strategy")
	}

	return nil
}

func (o *DeploymentStrategyOptions) Run() error {
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
		switch obj := info.Object.(type) {
		case *deployapi.DeploymentConfig:
			return o.updateDeploymentConfig(obj)
		case *extensions.Deployment:
			return o.updateDeployment(obj)
		case *extensions.DaemonSet:
			return o.updateDaemonSet(obj)
		case *apps.StatefulSet:
			return o.updateStatefulSet(obj)
		default:
			return false, fmt.Errorf("%s does not support deployment strategy", info.Mapping.Resource)
		}
	})

	if singleItemImplied && len(patches) == 0 {
		return fmt.Errorf("%s/%s was not updated", infos[0].Mapping.Resource, infos[0].Name)
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

func (o *DeploymentStrategyOptions) applyDeploymentConfigRecreateStrategyOptions(s *deployapi.RecreateDeploymentStrategyParams) bool {
	if o.Cmd.Flags().Changed("timeout-seconds") {
		s.TimeoutSeconds = &o.TimeoutSeconds
		return true
	}
	return false
}

func (o *DeploymentStrategyOptions) applyDeploymentConfigRollingStrategyOptions(s *deployapi.RollingDeploymentStrategyParams) bool {
	updated := false
	if o.Cmd.Flags().Changed("timeout-seconds") {
		s.TimeoutSeconds = &o.TimeoutSeconds
		updated = true
	}
	if o.Cmd.Flags().Changed("max-surge") {
		s.MaxSurge = intstr.FromString(o.MaxSurge)
		updated = true
	}
	if o.Cmd.Flags().Changed("max-unavailable") {
		glog.Infof("flag set")
		s.MaxUnavailable = intstr.FromString(o.MaxUnavailable)
		updated = true
	}
	if o.Cmd.Flags().Changed("interval-seconds") {
		s.IntervalSeconds = &o.IntervalSeconds
		updated = true
	}
	if o.Cmd.Flags().Changed("update-period-seconds") {
		s.UpdatePeriodSeconds = &o.UpdatePeriodSeconds
		updated = true
	}
	return updated
}

func (o *DeploymentStrategyOptions) applyDeploymentConfigCustomStrategyOptions(s *deployapi.CustomDeploymentStrategyParams) bool {
	updated := false
	if len(o.Command) > 0 {
		s.Command = o.Command
		updated = true
	}
	if o.Cmd.Flags().Changed("environment") {
		env, _, err := cmdutil.ParseEnv(o.Environment, nil)
		if err == nil {
			s.Environment = env
		}
		updated = true
	}
	if o.Cmd.Flags().Changed("image") {
		s.Image = o.Image
		updated = true
	}
	return updated
}

func (o *DeploymentStrategyOptions) updateDeploymentConfig(dc *deployapi.DeploymentConfig) (bool, error) {
	var (
		strategyChanged bool
		paramsChanged   bool
	)
	if o.Recreate {
		dc.Spec.Strategy.CustomParams = nil
		if dc.Spec.Strategy.Type != deployapi.DeploymentStrategyTypeRecreate {
			dc.Spec.Strategy.Type = deployapi.DeploymentStrategyTypeRecreate
			strategyChanged = true
		}
		if dc.Spec.Strategy.RecreateParams == nil {
			dc.Spec.Strategy.RecreateParams = &deployapi.RecreateDeploymentStrategyParams{}
			strategyChanged = true
		}
		if dc.Spec.Strategy.RollingParams != nil {
			// Preserve previous strategy parameters that are applicable
			dc.Spec.Strategy.RecreateParams.TimeoutSeconds = dc.Spec.Strategy.RollingParams.TimeoutSeconds
			dc.Spec.Strategy.RecreateParams.Pre = dc.Spec.Strategy.RollingParams.Pre
			dc.Spec.Strategy.RecreateParams.Post = dc.Spec.Strategy.RollingParams.Post
			// Delete the previous strategy
			dc.Spec.Strategy.RollingParams = nil
		}
		paramsChanged = o.applyDeploymentConfigRecreateStrategyOptions(dc.Spec.Strategy.RecreateParams)
	}

	if o.Rolling {
		dc.Spec.Strategy.CustomParams = nil
		if dc.Spec.Strategy.Type != deployapi.DeploymentStrategyTypeRolling {
			dc.Spec.Strategy.Type = deployapi.DeploymentStrategyTypeRolling
			strategyChanged = true
		}
		if dc.Spec.Strategy.RollingParams == nil {
			dc.Spec.Strategy.RollingParams = &deployapi.RollingDeploymentStrategyParams{}
			// FIXME: This is taken from defaults as for some reason the patch will
			// use the 0 values for both when we creating this from scratch.
			// This should be fixed when we switch to versioned API.
			dc.Spec.Strategy.RollingParams.MaxUnavailable = intstr.FromString("25%")
			dc.Spec.Strategy.RollingParams.MaxSurge = intstr.FromString("25%")
			strategyChanged = true
		}

		if dc.Spec.Strategy.RecreateParams != nil {
			// Preserve previous strategy parameters that are applicable
			dc.Spec.Strategy.RollingParams.TimeoutSeconds = dc.Spec.Strategy.RecreateParams.TimeoutSeconds
			dc.Spec.Strategy.RollingParams.Pre = dc.Spec.Strategy.RecreateParams.Pre
			dc.Spec.Strategy.RollingParams.Post = dc.Spec.Strategy.RecreateParams.Post

			// Tell users we are dropping the mid hook
			if dc.Spec.Strategy.RecreateParams.Mid != nil {
				fmt.Fprintf(o.Err, "warning: the mid hook is not supported by rolling strategy")
			}

			// Delete the previous strategy
			dc.Spec.Strategy.RecreateParams = nil
		}

		paramsChanged = o.applyDeploymentConfigRollingStrategyOptions(dc.Spec.Strategy.RollingParams)
	}

	if o.Custom {
		dc.Spec.Strategy.RollingParams = nil
		dc.Spec.Strategy.RecreateParams = nil

		if dc.Spec.Strategy.Type != deployapi.DeploymentStrategyTypeCustom {
			dc.Spec.Strategy.Type = deployapi.DeploymentStrategyTypeCustom
			strategyChanged = true
		}
		dc.Spec.Strategy.CustomParams = &deployapi.CustomDeploymentStrategyParams{}
		strategyChanged = true
		paramsChanged = o.applyDeploymentConfigCustomStrategyOptions(dc.Spec.Strategy.CustomParams)
	}

	return strategyChanged || paramsChanged, nil
}

func (o *DeploymentStrategyOptions) applyDeploymentRollingUpdateOptions(s *extensions.RollingUpdateDeployment) bool {
	updated := false
	if o.Cmd.Flags().Changed("max-surge") {
		s.MaxSurge = intstr.FromString(o.MaxSurge)
		updated = true
	}
	if o.Cmd.Flags().Changed("max-unavailable") {
		s.MaxUnavailable = intstr.FromString(o.MaxUnavailable)
		updated = true
	}
	return updated
}

func (o *DeploymentStrategyOptions) updateDeployment(d *extensions.Deployment) (bool, error) {
	var (
		strategyChanged bool
		paramsChanged   bool
	)
	if o.Recreate {
		d.Spec.Strategy.Type = extensions.RecreateDeploymentStrategyType
		d.Spec.Strategy.RollingUpdate = nil
		strategyChanged = true
	}
	if o.Rolling {
		d.Spec.Strategy.Type = extensions.RollingUpdateDeploymentStrategyType
		if d.Spec.Strategy.RollingUpdate == nil {
			d.Spec.Strategy.RollingUpdate = &extensions.RollingUpdateDeployment{}
			// FIXME: This is taken from defaults as for some reason the patch will
			// use the 0 values for both when we creating this from scratch.
			// This should be fixed when we switch to versioned API.
			d.Spec.Strategy.RollingUpdate.MaxUnavailable = intstr.FromString("25%")
			d.Spec.Strategy.RollingUpdate.MaxSurge = intstr.FromString("25%")
			strategyChanged = true
		}
		paramsChanged = o.applyDeploymentRollingUpdateOptions(d.Spec.Strategy.RollingUpdate)
	}
	return strategyChanged || paramsChanged, nil
}

func (o *DeploymentStrategyOptions) applyStatefulSetRollingUpdateOptions(s *apps.RollingUpdateStatefulSetStrategy) bool {
	updated := false
	if o.Cmd.Flags().Changed("partition") {
		s.Partition = o.Partition
		updated = true
	}
	return updated
}

func (o *DeploymentStrategyOptions) updateStatefulSet(s *apps.StatefulSet) (bool, error) {
	var (
		strategyChanged bool
		paramsChanged   bool
	)
	if o.OnDelete {
		s.Spec.UpdateStrategy.Type = apps.OnDeleteStatefulSetStrategyType
		s.Spec.UpdateStrategy.RollingUpdate = nil
		strategyChanged = true
	}
	if o.Rolling {
		s.Spec.UpdateStrategy.Type = apps.RollingUpdateStatefulSetStrategyType
		if s.Spec.UpdateStrategy.RollingUpdate == nil {
			s.Spec.UpdateStrategy.RollingUpdate = &apps.RollingUpdateStatefulSetStrategy{}
			strategyChanged = true
		}
		paramsChanged = o.applyStatefulSetRollingUpdateOptions(s.Spec.UpdateStrategy.RollingUpdate)
	}
	return strategyChanged || paramsChanged, nil
}

func (o *DeploymentStrategyOptions) applyDaemonSetRollingUpdateOptions(s *extensions.RollingUpdateDaemonSet) bool {
	updated := false
	if o.Cmd.Flags().Changed("max-unavailable") {
		s.MaxUnavailable = intstr.FromString(o.MaxUnavailable)
		updated = true
	}
	return updated
}

func (o *DeploymentStrategyOptions) updateDaemonSet(d *extensions.DaemonSet) (bool, error) {
	var (
		strategyChanged bool
		paramsChanged   bool
	)
	if o.OnDelete {
		d.Spec.UpdateStrategy.Type = extensions.OnDeleteDaemonSetStrategyType
		d.Spec.UpdateStrategy.RollingUpdate = nil
		strategyChanged = true
	}
	if o.Rolling {
		d.Spec.UpdateStrategy.Type = extensions.RollingUpdateDaemonSetStrategyType
		if d.Spec.UpdateStrategy.RollingUpdate == nil {
			d.Spec.UpdateStrategy.RollingUpdate = &extensions.RollingUpdateDaemonSet{}
			// FIXME: This is taken from defaults as for some reason the patch will
			// use the 0 values for both when we creating this from scratch.
			// This should be fixed when we switch to versioned API.
			d.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable = intstr.FromString("25%")
			strategyChanged = true
		}
		paramsChanged = o.applyDaemonSetRollingUpdateOptions(d.Spec.UpdateStrategy.RollingUpdate)
	}
	return strategyChanged || paramsChanged, nil
}
