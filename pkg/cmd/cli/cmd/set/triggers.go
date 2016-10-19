package set

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/generate/app"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"k8s.io/kubernetes/pkg/util/sets"
)

var (
	triggersLong = templates.LongDesc(`
		Set or remove triggers for build configs and deployment configs

		All build configs and deployment configs may have a set of triggers that result in a new deployment
		or build being created. This command enables you to alter those triggers - making them automatic or
		manual, adding new entries, or changing existing entries.

		Deployments support triggering off of image changes and on config changes. Config changes are any
		alterations to the pod template, while image changes will result in the container image value being
		updated whenever an image stream tag is updated.

		Build configs support triggering off of image changes, config changes, and webhooks (both GitHub-specific
		and generic). The config change trigger for a build config will only trigger the first build.`)

	triggersExample = templates.Examples(`
		# Print the triggers on the registry
	  %[1]s triggers dc/registry

	  # Set all triggers to manual
	  %[1]s triggers dc/registry --manual

	  # Enable all automatic triggers
	  %[1]s triggers dc/registry --auto

	  # Reset the GitHub webhook on a build to a new, generated secret
	  %[1]s triggers bc/webapp --from-github
	  %[1]s triggers bc/webapp --from-webhook

	  # Remove all triggers
	  %[1]s triggers bc/webapp --remove-all

	  # Stop triggering on config change
	  %[1]s triggers dc/registry --from-config --remove

	  # Add an image trigger to a build config
	  %[1]s triggers bc/webapp --from-image=namespace1/image:latest`)
)

type TriggersOptions struct {
	Out io.Writer
	Err io.Writer

	Filenames []string
	Selector  string
	All       bool

	Builder *resource.Builder
	Infos   []*resource.Info

	Encoder runtime.Encoder

	ShortOutput   bool
	Mapper        meta.RESTMapper
	OutputVersion unversioned.GroupVersion

	PrintTable  bool
	PrintObject func([]*resource.Info) error

	Remove    bool
	RemoveAll bool
	Auto      bool
	Manual    bool
	Reset     bool

	ContainerNames      string
	FromConfig          bool
	FromGitHub          *bool
	FromWebHook         *bool
	FromWebHookAllowEnv *bool
	FromImage           string
	// FromImageNamespace is the namespace for the FromImage
	FromImageNamespace string
}

// NewCmdTriggers implements the set triggers command
func NewCmdTriggers(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &TriggersOptions{
		Out: out,
		Err: errOut,
	}
	cmd := &cobra.Command{
		Use:     "triggers RESOURCE/NAME [--from-config|--from-image|--from-github|--from-webhook] [--auto|--manual]",
		Short:   "Update the triggers on a build or deployment config",
		Long:    triggersLong,
		Example: fmt.Sprintf(triggersExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			if err := options.Run(); err != nil {
				// TODO: move met to kcmdutil
				if err == cmdutil.ErrExit {
					os.Exit(1)
				}
				kcmdutil.CheckErr(err)
			}
		},
	}

	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&options.All, "all", options.All, "Select all resources in the namespace of the specified resource types")
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")

	cmd.Flags().BoolVar(&options.Remove, "remove", options.Remove, "If true, remove the specified trigger(s).")
	cmd.Flags().BoolVar(&options.RemoveAll, "remove-all", options.RemoveAll, "If true, remove all triggers.")
	cmd.Flags().BoolVar(&options.Auto, "auto", options.Auto, "Enable all triggers, or just the specified trigger")
	cmd.Flags().BoolVar(&options.Manual, "manual", options.Manual, "Set all triggers to manual, or just the specified trigger")

	cmd.Flags().BoolVar(&options.FromConfig, "from-config", options.FromConfig, "If set, configuration changes will result in a change")
	cmd.Flags().StringVarP(&options.ContainerNames, "containers", "c", options.ContainerNames, "Comma delimited list of container names this trigger applies to on deployments; defaults to the name of the only container")
	cmd.Flags().StringVar(&options.FromImage, "from-image", options.FromImage, "An image stream tag to trigger off of")
	options.FromGitHub = cmd.Flags().Bool("from-github", false, "A GitHub webhook - a secret value will be generated automatically")
	options.FromWebHook = cmd.Flags().Bool("from-webhook", false, "A generic webhook - a secret value will be generated automatically")
	options.FromWebHookAllowEnv = cmd.Flags().Bool("from-webhook-allow-env", false, "A generic webhook which can provide environment variables - a secret value will be generated automatically")

	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")

	return cmd
}

func (o *TriggersOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	o.OutputVersion, err = kcmdutil.OutputVersion(cmd, clientConfig.GroupVersion)
	if err != nil {
		return err
	}

	if !cmd.Flags().Lookup("from-github").Changed {
		o.FromGitHub = nil
	}
	if !cmd.Flags().Lookup("from-webhook").Changed {
		o.FromWebHook = nil
	}
	if !cmd.Flags().Lookup("from-webhook-allow-env").Changed {
		o.FromWebHookAllowEnv = nil
	}

	if len(o.FromImage) > 0 {
		ref, err := imageapi.ParseDockerImageReference(o.FromImage)
		if err != nil {
			return fmt.Errorf("the value of --from-image does not appear to be a valid reference to an image: %v", err)
		}
		if len(ref.Registry) > 0 || len(ref.ID) > 0 {
			return fmt.Errorf("the value of --from-image must point to an image stream tag on this server")
		}
		if len(ref.Tag) == 0 {
			return fmt.Errorf("the value of --from-image must include the tag you wish to pull from")
		}
		o.FromImage = ref.NameString()
		o.FromImageNamespace = defaultNamespace(ref.Namespace, cmdNamespace)
	}

	count := o.count()
	o.Reset = count == 0 && (o.Auto || o.Manual)
	switch {
	case count == 0 && !o.Remove && !o.RemoveAll && !o.Auto && !o.Manual:
		o.PrintTable = true
	case !o.RemoveAll && !o.Auto && !o.Manual:
		o.Auto = true
	}

	mapper, typer := f.Object(false)
	o.Builder = resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, false, o.Filenames...).
		SelectorParam(o.Selector).
		ResourceTypeOrNameArgs(o.All, args...).
		Flatten()

	output := kcmdutil.GetFlagString(cmd, "output")
	if len(output) > 0 {
		o.PrintObject = func(infos []*resource.Info) error {
			return f.PrintResourceInfos(cmd, infos, o.Out)
		}
	}

	o.Encoder = f.JSONEncoder()
	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"
	o.Mapper = mapper

	return nil
}

func (o *TriggersOptions) count() int {
	count := 0
	if o.FromConfig {
		count++
	}
	if o.FromGitHub != nil {
		count++
	}
	if o.FromWebHook != nil {
		count++
	}
	if o.FromWebHookAllowEnv != nil {
		count++
	}
	if len(o.FromImage) > 0 {
		count++
	}
	return count
}

func (o *TriggersOptions) Validate() error {
	count := o.count()
	switch {
	case o.Auto && o.Manual:
		return fmt.Errorf("you must specify at most one of --auto or --manual")
	case o.Remove && o.RemoveAll:
		return fmt.Errorf("you must specify either --remove or --remove-all")
	case o.RemoveAll && (count != 0 || o.Auto || o.Manual):
		return fmt.Errorf("--remove-all may not be used with any other flag")
	case o.Remove && count < 1:
		return fmt.Errorf("--remove requires a flag defining a trigger type to be specified")
	case count > 1:
		return fmt.Errorf("you may only set one trigger type at a time")
	case count == 0 && !o.Remove && !o.RemoveAll && !o.Auto && !o.Manual && !o.PrintTable:
		return fmt.Errorf("specify one of the --from-* flags to add a trigger, --remove to remove, or --auto|--manual to control existing triggers")
	}
	return nil
}

func (o *TriggersOptions) Run() error {
	infos := o.Infos
	singular := len(o.Infos) <= 1
	if o.Builder != nil {
		loaded, err := o.Builder.Do().IntoSingular(&singular).Infos()
		if err != nil {
			return err
		}
		infos = loaded
	}

	if o.PrintTable && o.PrintObject == nil {
		return o.printTriggers(infos)
	}

	updateTriggerFn := func(triggers *TriggerDefinition) error {
		o.updateTriggers(triggers)
		return nil
	}
	patches := CalculatePatches(infos, o.Encoder, func(info *resource.Info) (bool, error) {
		return UpdateTriggersForObject(info.Object, updateTriggerFn)
	})
	if singular && len(patches) == 0 {
		return fmt.Errorf("%s/%s is not a deployment config or build config", infos[0].Mapping.Resource, infos[0].Name)
	}
	if o.PrintObject != nil {
		return o.PrintObject(infos)
	}

	failed := false
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			failed = true
			fmt.Fprintf(o.Err, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, patch.Err)
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			fmt.Fprintf(o.Err, "info: %s %q was not changed\n", info.Mapping.Resource, info.Name)
			continue
		}

		glog.V(4).Infof("Calculated patch %s", patch.Patch)

		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, kapi.StrategicMergePatchType, patch.Patch)
		if err != nil {
			handlePodUpdateError(o.Err, err, "triggered")
			failed = true
			continue
		}

		info.Refresh(obj, true)
		kcmdutil.PrintSuccess(o.Mapper, o.ShortOutput, o.Out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if failed {
		return cmdutil.ErrExit
	}
	return nil
}

// printTriggers displays a tabular output of the triggers for each object.
func (o *TriggersOptions) printTriggers(infos []*resource.Info) error {
	w := tabwriter.NewWriter(o.Out, 0, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\tTYPE\tVALUE\tAUTO\n")
	for _, info := range infos {
		_, err := UpdateTriggersForObject(info.Object, func(triggers *TriggerDefinition) error {
			fmt.Fprintf(w, "%s/%s\t%s\t%s\t%t\n", info.Mapping.Resource, info.Name, "config", "", triggers.ConfigChange)
			for _, image := range triggers.ImageChange {
				var details string
				switch {
				case len(image.Names) > 0:
					if len(image.Namespace) > 0 {
						details = fmt.Sprintf("%s/%s (%s)", image.Namespace, image.From, strings.Join(image.Names, ", "))
					} else {
						details = fmt.Sprintf("%s (%s)", image.From, strings.Join(image.Names, ", "))
					}
				case len(image.Namespace) > 0:
					details = fmt.Sprintf("%s/%s", image.Namespace, image.From)
				default:
					details = image.From
				}
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%t\n", info.Mapping.Resource, info.Name, "image", details, image.Auto)
			}
			for _, s := range triggers.WebHooks {
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\n", info.Mapping.Resource, info.Name, "webhook", s, "")
			}
			for _, s := range triggers.GitHubWebHooks {
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\n", info.Mapping.Resource, info.Name, "github", s, "")
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(w, "%s/%s\t%s\t%s\t%t\n", info.Mapping.Resource, info.Name, "<error>", "", false)
		}
	}
	return nil
}

// updateTriggers updates only those fields with flags set by the user
func (o *TriggersOptions) updateTriggers(triggers *TriggerDefinition) {
	// clear everything
	if o.RemoveAll {
		*triggers = TriggerDefinition{}
		return
	}

	// clear a specific field
	if o.Remove {
		if o.FromConfig {
			triggers.ConfigChange = false
		}
		if len(o.FromImage) > 0 {
			var newTriggers []ImageChangeTrigger
			for _, trigger := range triggers.ImageChange {
				if trigger.From != o.FromImage {
					newTriggers = append(newTriggers, trigger)
				}
			}
			triggers.ImageChange = newTriggers
		}
		if o.FromWebHook != nil && *o.FromWebHook {
			triggers.WebHooks = nil
		}
		if o.FromWebHookAllowEnv != nil && *o.FromWebHookAllowEnv {
			triggers.WebHooks = nil
			triggers.WebHooksAllowEnv = false
		}
		if o.FromGitHub != nil && *o.FromGitHub {
			triggers.GitHubWebHooks = nil
		}
		return
	}

	// change the automated status
	if o.Reset {
		triggers.ConfigChange = o.Auto
		for i := range triggers.ImageChange {
			triggers.ImageChange[i].Auto = o.Auto
		}
		return
	}

	// change individual elements
	if o.FromConfig {
		triggers.ConfigChange = true
	}
	if len(o.FromImage) > 0 {
		names := strings.Split(o.ContainerNames, ",")
		if len(o.ContainerNames) == 0 {
			names = nil
		}
		found := false
		for i, trigger := range triggers.ImageChange {
			if trigger.From == o.FromImage && trigger.Namespace == o.FromImageNamespace {
				found = true
				triggers.ImageChange[i].Auto = !o.Manual
				triggers.ImageChange[i].Names = names
				break
			}
		}
		if !found {
			triggers.ImageChange = append(triggers.ImageChange, ImageChangeTrigger{
				From:      o.FromImage,
				Namespace: o.FromImageNamespace,
				Auto:      !o.Manual,
				Names:     names,
			})
		}
	}
	if o.FromWebHook != nil && *o.FromWebHook {
		triggers.WebHooks = []string{app.GenerateSecret(20)}
	}
	if o.FromWebHookAllowEnv != nil && *o.FromWebHookAllowEnv {
		triggers.WebHooks = []string{app.GenerateSecret(20)}
		triggers.WebHooksAllowEnv = true
	}
	if o.FromGitHub != nil && *o.FromGitHub {
		triggers.GitHubWebHooks = []string{app.GenerateSecret(20)}
	}
}

// ImageChangeTrigger represents the capabilities present in deployment config and build
// config objects in a consistent way.
type ImageChangeTrigger struct {
	// If this trigger is automatically applied
	Auto bool
	// An ImageStreamTag name to target
	From string
	// The target namespace, normalized if set
	Namespace string
	// A list of names this trigger targets
	Names []string
}

// TriggerDefinition is the abstract representation of triggers for builds and deploymnet configs.
type TriggerDefinition struct {
	ConfigChange     bool
	ImageChange      []ImageChangeTrigger
	WebHooks         []string
	WebHooksAllowEnv bool
	GitHubWebHooks   []string
}

// defaultNamespace returns an empty string if the provided namespace matches the default namespace, or
// returns the namespace.
func defaultNamespace(namespace, defaultNamespace string) string {
	if namespace == defaultNamespace {
		return ""
	}
	return namespace
}

// NewDeploymentConfigTriggers creates a trigger definition from a deployment config.
func NewDeploymentConfigTriggers(config *deployapi.DeploymentConfig) *TriggerDefinition {
	t := &TriggerDefinition{}
	for _, trigger := range config.Spec.Triggers {
		switch trigger.Type {
		case deployapi.DeploymentTriggerOnConfigChange:
			t.ConfigChange = true
		case deployapi.DeploymentTriggerOnImageChange:
			t.ImageChange = append(t.ImageChange, ImageChangeTrigger{
				Auto:      trigger.ImageChangeParams.Automatic,
				Names:     trigger.ImageChangeParams.ContainerNames,
				From:      trigger.ImageChangeParams.From.Name,
				Namespace: defaultNamespace(trigger.ImageChangeParams.From.Namespace, config.Namespace),
			})
		}
	}
	return t
}

// NewBuildConfigTriggers creates a trigger definition from a build config.
func NewBuildConfigTriggers(config *buildapi.BuildConfig) *TriggerDefinition {
	t := &TriggerDefinition{}
	setStrategy := false
	for _, trigger := range config.Spec.Triggers {
		switch trigger.Type {
		case buildapi.ConfigChangeBuildTriggerType:
			t.ConfigChange = true
		case buildapi.GenericWebHookBuildTriggerType:
			t.WebHooks = append(t.WebHooks, trigger.GenericWebHook.Secret)
			t.WebHooksAllowEnv = trigger.GenericWebHook.AllowEnv
		case buildapi.GitHubWebHookBuildTriggerType:
			t.GitHubWebHooks = append(t.GitHubWebHooks, trigger.GitHubWebHook.Secret)
		case buildapi.ImageChangeBuildTriggerType:
			if trigger.ImageChange.From == nil {
				if strategyTrigger := strategyTrigger(config); strategyTrigger != nil {
					setStrategy = true
					strategyTrigger.Auto = true
					t.ImageChange = append(t.ImageChange, *strategyTrigger)
				}
				continue
			}
			// normalize the trigger
			trigger.ImageChange.From.Namespace = defaultNamespace(trigger.ImageChange.From.Namespace, config.Namespace)
			t.ImageChange = append(t.ImageChange, ImageChangeTrigger{
				Auto:      true,
				From:      trigger.ImageChange.From.Name,
				Namespace: trigger.ImageChange.From.Namespace,
			})
		}
	}
	if !setStrategy {
		if strategyTrigger := strategyTrigger(config); strategyTrigger != nil {
			t.ImageChange = append(t.ImageChange, *strategyTrigger)
		}
	}
	return t
}

// Apply writes a trigger definition back to a build or deployment config.
func (t *TriggerDefinition) Apply(obj runtime.Object) error {
	switch c := obj.(type) {
	case *deployapi.DeploymentConfig:
		if len(t.GitHubWebHooks) > 0 {
			return fmt.Errorf("deployment configs do not support GitHub web hooks")
		}
		if len(t.WebHooks) > 0 {
			return fmt.Errorf("deployment configs do not support web hooks")
		}

		existingTriggers := filterDeploymentTriggers(c.Spec.Triggers, deployapi.DeploymentTriggerOnConfigChange)
		var triggers []deployapi.DeploymentTriggerPolicy
		if t.ConfigChange {
			triggers = append(triggers, deployapi.DeploymentTriggerPolicy{Type: deployapi.DeploymentTriggerOnConfigChange})
		}
		allNames := sets.NewString()
		for _, container := range c.Spec.Template.Spec.Containers {
			allNames.Insert(container.Name)
		}
		for _, trigger := range t.ImageChange {
			if len(trigger.Names) == 0 {
				return fmt.Errorf("you must specify --containers when setting --from-image")
			}
			if !allNames.HasAll(trigger.Names...) {
				return fmt.Errorf(
					"not all container names exist: %s (accepts: %s)",
					strings.Join(sets.NewString(trigger.Names...).Difference(allNames).List(), ", "),
					strings.Join(allNames.List(), ", "),
				)
			}
			triggers = append(triggers, deployapi.DeploymentTriggerPolicy{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					Automatic: trigger.Auto,
					From: kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: trigger.From,
					},
					ContainerNames: trigger.Names,
				},
			})
		}
		c.Spec.Triggers = mergeDeployTriggers(existingTriggers, triggers)
		return nil

	case *buildapi.BuildConfig:
		var triggers []buildapi.BuildTriggerPolicy
		if t.ConfigChange {
			triggers = append(triggers, buildapi.BuildTriggerPolicy{Type: buildapi.ConfigChangeBuildTriggerType})
		}
		for _, trigger := range t.WebHooks {
			triggers = append(triggers, buildapi.BuildTriggerPolicy{
				Type: buildapi.GenericWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret:   trigger,
					AllowEnv: t.WebHooksAllowEnv,
				},
			})
		}
		for _, trigger := range t.GitHubWebHooks {
			triggers = append(triggers, buildapi.BuildTriggerPolicy{
				Type: buildapi.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &buildapi.WebHookTrigger{
					Secret: trigger,
				},
			})
		}

		// add new triggers, filter out any old triggers that match (if moving from automatic to manual),
		// and then merge the old triggers and the new triggers to preserve fields like lastTriggeredImageID
		existingTriggers := c.Spec.Triggers
		strategyTrigger := strategyTrigger(c)
		for _, trigger := range t.ImageChange {
			change := &buildapi.ImageChangeTrigger{
				From: &kapi.ObjectReference{
					Kind:      "ImageStreamTag",
					Name:      trigger.From,
					Namespace: trigger.Namespace,
				},
			}

			// use the canonical ImageChangeTrigger with nil From
			strategyTrigger.Auto = trigger.Auto
			if reflect.DeepEqual(strategyTrigger, &trigger) {
				change.From = nil
			}

			// if this trigger is not automatic, then we need to remove it from the list of triggers
			if !trigger.Auto {
				existingTriggers = filterBuildImageTriggers(existingTriggers, trigger, strategyTrigger)
				continue
			}

			triggers = append(triggers, buildapi.BuildTriggerPolicy{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: change,
			})
		}
		c.Spec.Triggers = mergeBuildTriggers(existingTriggers, triggers)
		return nil

	default:
		return fmt.Errorf("the object is not a deployment config or build config")
	}
}

// triggerMatchesBuildImageChange identifies whether the image change is equivalent to the trigger
func triggerMatchesBuildImageChange(trigger ImageChangeTrigger, strategyTrigger *ImageChangeTrigger, imageChange *buildapi.ImageChangeTrigger) bool {
	if imageChange == nil {
		return false
	}
	if imageChange.From == nil {
		return strategyTrigger != nil && strategyTrigger.From == trigger.From && strategyTrigger.Namespace == trigger.Namespace
	}
	namespace := imageChange.From.Namespace
	if strategyTrigger != nil {
		namespace = defaultNamespace(namespace, strategyTrigger.Namespace)
	}
	return imageChange.From.Name == trigger.From && namespace == trigger.Namespace
}

// filterBuildImageTriggers return only triggers that do not match the provided ImageChangeTrigger.  strategyTrigger may be provided
// if set to remove a BuildTriggerPolicy without a From (which points to the strategy)
func filterBuildImageTriggers(src []buildapi.BuildTriggerPolicy, trigger ImageChangeTrigger, strategyTrigger *ImageChangeTrigger) []buildapi.BuildTriggerPolicy {
	var dst []buildapi.BuildTriggerPolicy
	for i := range src {
		if triggerMatchesBuildImageChange(trigger, strategyTrigger, src[i].ImageChange) {
			continue
		}
		dst = append(dst, src[i])
	}
	return dst
}

// filterDeploymentTriggers returns only triggers that do not have one of the provided types.
func filterDeploymentTriggers(src []deployapi.DeploymentTriggerPolicy, types ...deployapi.DeploymentTriggerType) []deployapi.DeploymentTriggerPolicy {
	var dst []deployapi.DeploymentTriggerPolicy
Outer:
	for i := range src {
		for _, t := range types {
			if t == src[i].Type {
				continue Outer
			}
		}
		dst = append(dst, src[i])
	}
	return dst
}

// strategyTrigger returns a synthetic ImageChangeTrigger that represents the image stream tag the build strategy
// points to, or nil if no such strategy trigger is possible (if the build doesn't point to an ImageStreamTag).
func strategyTrigger(config *buildapi.BuildConfig) *ImageChangeTrigger {
	if from := buildutil.GetInputReference(config.Spec.Strategy); from != nil {
		if from.Kind == "ImageStreamTag" {
			// normalize the strategy object reference
			from.Namespace = defaultNamespace(from.Namespace, config.Namespace)
			return &ImageChangeTrigger{From: from.Name, Namespace: from.Namespace}
		}
	}
	return nil
}

// mergeDeployTriggers returns an array of DeploymentTriggerPolicies that have no duplicates.
func mergeDeployTriggers(dst, src []deployapi.DeploymentTriggerPolicy) []deployapi.DeploymentTriggerPolicy {
	// never return an empty map, because the triggers on a deployment config default when the map is empty
	result := []deployapi.DeploymentTriggerPolicy{}
	for _, current := range dst {
		if findDeployTrigger(src, current) != -1 {
			result = append(result, current)
		}
	}
	for _, current := range src {
		if findDeployTrigger(result, current) == -1 {
			result = append(result, current)
		}
	}
	return result
}

// findDeployTrigger finds the position of a deployment trigger in the provided array, or -1 if no such
// matching trigger is found.
func findDeployTrigger(dst []deployapi.DeploymentTriggerPolicy, trigger deployapi.DeploymentTriggerPolicy) int {
	for i := range dst {
		if reflect.DeepEqual(dst[i], trigger) {
			return i
		}
	}
	return -1
}

// mergeBuildTriggers returns an array of BuildTriggerPolicies that have no duplicates, in the same order
// as they exist in their original arrays (a zip-merge).
func mergeBuildTriggers(dst, src []buildapi.BuildTriggerPolicy) []buildapi.BuildTriggerPolicy {
	var result []buildapi.BuildTriggerPolicy
	for _, current := range dst {
		if findBuildTrigger(src, current) != -1 {
			result = append(result, current)
		}
	}
	for _, current := range src {
		if findBuildTrigger(result, current) == -1 {
			result = append(result, current)
		}
	}
	return result
}

// findBuildTrigger finds the equivalent build trigger position in the provided array, or -1 if
// no such build trigger exists.  Equality only cares about the value of the From field.
func findBuildTrigger(dst []buildapi.BuildTriggerPolicy, trigger buildapi.BuildTriggerPolicy) int {
	// make a copy for semantic equality
	if trigger.ImageChange != nil {
		trigger.ImageChange = &buildapi.ImageChangeTrigger{From: trigger.ImageChange.From}
	}
	for i, copied := range dst {
		// make a copy for semantic equality
		if copied.ImageChange != nil {
			copied.ImageChange = &buildapi.ImageChangeTrigger{From: copied.ImageChange.From}
		}
		if reflect.DeepEqual(copied, trigger) {
			return i
		}
	}
	return -1
}

// UpdateTriggersForObject extracts a trigger definition from the provided object, passes it to fn, and
// then applies the trigger definition back on the object. It returns true if the object was mutated
// and an optional error if the any part of the flow returns error.
func UpdateTriggersForObject(obj runtime.Object, fn func(*TriggerDefinition) error) (bool, error) {
	// TODO: replace with a swagger schema based approach (identify pod template via schema introspection)
	switch t := obj.(type) {
	case *deployapi.DeploymentConfig:
		triggers := NewDeploymentConfigTriggers(t)
		if err := fn(triggers); err != nil {
			return true, err
		}
		return true, triggers.Apply(t)
	case *buildapi.BuildConfig:
		triggers := NewBuildConfigTriggers(t)
		if err := fn(triggers); err != nil {
			return true, err
		}
		return true, triggers.Apply(t)
	default:
		return false, fmt.Errorf("the object is not a deployment config or build config")
	}
}
