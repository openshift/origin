package set

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kappsv1 "k8s.io/api/apps/v1"
	kappsv1beta1 "k8s.io/api/apps/v1beta1"
	kappsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	batchv2alpha1 "k8s.io/api/batch/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	ometa "github.com/openshift/origin/pkg/api/imagereferencemutators"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	triggerapi "github.com/openshift/origin/pkg/image/apis/image/v1/trigger"
	"github.com/openshift/origin/pkg/image/trigger/annotations"
	"github.com/openshift/origin/pkg/oc/lib/newapp/app"
)

var (
	triggersLong = templates.LongDesc(`
		Set or remove triggers

		Build configs, deployment configs, and most Kubernetes workload objects may have a set of triggers
		that result in a new deployment or build being created when an image changes. This command enables
		you to alter those triggers - making them automatic or manual, adding new entries, or changing
		existing entries.

		Deployments support triggering off of image changes and on config changes. Config changes are any
		alterations to the pod template, while image changes will result in the container image value being
		updated whenever an image stream tag is updated. You may also trigger Kubernetes stateful sets,
		daemon sets, deployments, and cron jobs from images. Disabling the config change trigger is equivalent
		to pausing most objects. Deployment configs will not perform their first deployment until all image
		change triggers have been submitted.

		Build configs support triggering off of image changes, config changes, and webhooks. The config change
		trigger for a build config will only trigger the first build.`)

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
	  %[1]s triggers bc/webapp --from-image=namespace1/image:latest

	  # Add an image trigger to a stateful set on the main container
	  %[1]s triggers statefulset/db --from-image=namespace1/image:latest -c main`)
)

type TriggersOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Selector            string
	All                 bool
	Local               bool
	Remove              bool
	RemoveAll           bool
	Auto                bool
	Manual              bool
	Reset               bool
	ContainerNames      string
	FromConfig          bool
	FromImage           string
	FromGitHub          *bool
	FromWebHook         *bool
	FromWebHookAllowEnv *bool
	FromGitLab          *bool
	FromBitbucket       *bool
	// FromImageNamespace is the namespace for the FromImage
	FromImageNamespace string

	PrintTable        bool
	Client            dynamic.Interface
	Printer           printers.ResourcePrinter
	Builder           func() *resource.Builder
	Namespace         string
	ExplicitNamespace bool
	DryRun            bool
	Args              []string

	resource.FilenameOptions
	genericclioptions.IOStreams
}

func NewTriggersOptions(streams genericclioptions.IOStreams) *TriggersOptions {
	return &TriggersOptions{
		PrintFlags: genericclioptions.NewPrintFlags("triggers updated").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

// NewCmdTriggers implements the set triggers command
func NewCmdTriggers(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewTriggersOptions(streams)
	cmd := &cobra.Command{
		Use:     "triggers RESOURCE/NAME [--from-config|--from-image|--from-github|--from-webhook] [--auto|--manual]",
		Short:   "Update the triggers on one or more objects",
		Long:    triggersLong,
		Example: fmt.Sprintf(triggersExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	usage := "to use to edit the resource"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, select all resources in the namespace of the specified resource types")
	cmd.Flags().BoolVar(&o.Remove, "remove", o.Remove, "If true, remove the specified trigger(s).")
	cmd.Flags().BoolVar(&o.RemoveAll, "remove-all", o.RemoveAll, "If true, remove all triggers.")
	cmd.Flags().BoolVar(&o.Auto, "auto", o.Auto, "If true, enable all triggers, or just the specified trigger")
	cmd.Flags().BoolVar(&o.Manual, "manual", o.Manual, "If true, set all triggers to manual, or just the specified trigger")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set image will NOT contact api-server but run locally.")
	cmd.Flags().BoolVar(&o.FromConfig, "from-config", o.FromConfig, "If set, configuration changes will result in a change")
	cmd.Flags().StringVarP(&o.ContainerNames, "containers", "c", o.ContainerNames, "Comma delimited list of container names this trigger applies to on deployments; defaults to the name of the only container")
	cmd.Flags().StringVar(&o.FromImage, "from-image", o.FromImage, "An image stream tag to trigger off of")
	o.FromGitHub = cmd.Flags().Bool("from-github", false, "If true, a GitHub webhook - a secret value will be generated automatically")
	o.FromWebHook = cmd.Flags().Bool("from-webhook", false, "If true, a generic webhook - a secret value will be generated automatically")
	o.FromWebHookAllowEnv = cmd.Flags().Bool("from-webhook-allow-env", false, "If true, a generic webhook which can provide environment variables - a secret value will be generated automatically")
	o.FromGitLab = cmd.Flags().Bool("from-gitlab", false, "If true, a GitLab webhook - a secret value will be generated automatically")
	o.FromBitbucket = cmd.Flags().Bool("from-bitbucket", false, "If true, a Bitbucket webhook - a secret value will be generated automatically")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *TriggersOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
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
	if !cmd.Flags().Lookup("from-gitlab").Changed {
		o.FromGitLab = nil
	}
	if !cmd.Flags().Lookup("from-bitbucket").Changed {
		o.FromBitbucket = nil
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
		o.FromImageNamespace = defaultNamespace(ref.Namespace, o.Namespace)
	}

	count := o.count()
	o.Reset = count == 0 && (o.Auto || o.Manual)
	switch {
	case count == 0 && !o.Remove && !o.RemoveAll && !o.Auto && !o.Manual:
		o.PrintTable = true
	case !o.RemoveAll && !o.Auto && !o.Manual:
		o.Auto = true
	}

	o.Args = args
	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
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
	if o.FromGitLab != nil {
		count++
	}
	if o.FromBitbucket != nil {
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
	b := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		Flatten()

	if !o.Local {
		b = b.
			LabelSelectorParam(o.Selector).
			ResourceTypeOrNameArgs(o.All, o.Args...).
			Latest()
	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	if o.PrintTable {
		return o.printTriggers(infos)
	}

	updateTriggerFn := func(triggers *TriggerDefinition) error {
		o.updateTriggers(triggers)
		return nil
	}
	patches := CalculatePatchesExternal(infos, func(info *resource.Info) (bool, error) {
		return UpdateTriggersForObject(info.Object, updateTriggerFn)
	})
	if singleItemImplied && len(patches) == 0 {
		return fmt.Errorf("%s/%s does not support triggers", infos[0].Mapping.Resource.Resource, infos[0].Name)
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
			allErrs = append(allErrs, fmt.Errorf("failed to patch build hook: %v\n", err))
			continue
		}

		if err := o.Printer.PrintObj(actual, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)

}

// printTriggers displays a tabular output of the triggers for each object.
func (o *TriggersOptions) printTriggers(infos []*resource.Info) error {
	w := tabwriter.NewWriter(o.Out, 0, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\tTYPE\tVALUE\tAUTO\n")
	for _, info := range infos {
		_, err := UpdateTriggersForObject(info.Object, func(triggers *TriggerDefinition) error {
			fmt.Fprintf(w, "%s/%s\t%s\t%s\t%t\n", info.Mapping.Resource.Resource, info.Name, "config", "", triggers.ConfigChange)
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
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%t\n", info.Mapping.Resource.Resource, info.Name, "image", details, image.Auto)
			}
			for _, s := range triggers.GenericWebHooks {
				val := "<secret>"
				if s.AllowEnv {
					val += ", allowenv"
				}
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\n", info.Mapping.Resource.Resource, info.Name, "webhook", val, "")
			}
			for range triggers.GitHubWebHooks {
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\n", info.Mapping.Resource.Resource, info.Name, "github", "<secret>", "")
			}
			for range triggers.GitLabWebHooks {
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\n", info.Mapping.Resource.Resource, info.Name, "gitlab", "<secret>", "")
			}
			for range triggers.BitbucketWebHooks {
				fmt.Fprintf(w, "%s/%s\t%s\t%s\t%s\n", info.Mapping.Resource.Resource, info.Name, "bitbucket", "<secret>", "")
			}
			return nil
		})
		if err != nil {
			glog.V(2).Infof("Unable to calculate trigger for %s: %v", info.Name, err)
			fmt.Fprintf(w, "%s/%s\t%s\t%s\t%t\n", info.Mapping.Resource.Resource, info.Name, "<error>", "", false)
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
			triggers.GenericWebHooks = nil
		}
		if o.FromWebHookAllowEnv != nil && *o.FromWebHookAllowEnv {
			triggers.GenericWebHooks = nil
		}
		if o.FromGitHub != nil && *o.FromGitHub {
			triggers.GitHubWebHooks = nil
		}
		if o.FromGitLab != nil && *o.FromGitLab {
			triggers.GitLabWebHooks = nil
		}
		if o.FromBitbucket != nil && *o.FromBitbucket {
			triggers.BitbucketWebHooks = nil
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
		secret := app.GenerateSecret(20)
		triggers.GenericWebHooks = append(triggers.GenericWebHooks,
			buildv1.WebHookTrigger{
				Secret:   secret,
				AllowEnv: false,
			},
		)
	}
	if o.FromWebHookAllowEnv != nil && *o.FromWebHookAllowEnv {
		secret := app.GenerateSecret(20)
		triggers.GenericWebHooks = append(triggers.GenericWebHooks,
			buildv1.WebHookTrigger{
				Secret:   secret,
				AllowEnv: true,
			},
		)
	}
	if o.FromGitHub != nil && *o.FromGitHub {
		secret := app.GenerateSecret(20)
		triggers.GitHubWebHooks = append(triggers.GitHubWebHooks,
			buildv1.WebHookTrigger{
				Secret: secret,
			},
		)
	}
	if o.FromGitLab != nil && *o.FromGitLab {
		secret := app.GenerateSecret(20)
		triggers.GitLabWebHooks = append(triggers.GitLabWebHooks,
			buildv1.WebHookTrigger{
				Secret: secret,
			},
		)
	}
	if o.FromBitbucket != nil && *o.FromBitbucket {
		secret := app.GenerateSecret(20)
		triggers.BitbucketWebHooks = append(triggers.BitbucketWebHooks,
			buildv1.WebHookTrigger{
				Secret: secret,
			},
		)
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

// TriggerDefinition is the abstract representation of triggers for builds and deployment configs.
type TriggerDefinition struct {
	ConfigChange      bool
	ImageChange       []ImageChangeTrigger
	GenericWebHooks   []buildv1.WebHookTrigger
	GitHubWebHooks    []buildv1.WebHookTrigger
	GitLabWebHooks    []buildv1.WebHookTrigger
	BitbucketWebHooks []buildv1.WebHookTrigger
}

// defaultNamespace returns an empty string if the provided namespace matches the default namespace, or
// returns the namespace.
func defaultNamespace(namespace, defaultNamespace string) string {
	if namespace == defaultNamespace {
		return ""
	}
	return namespace
}

// NewAnnotationTriggers creates a trigger definition from an object that can be triggered by the image
// annotation.
func NewAnnotationTriggers(obj runtime.Object) (*TriggerDefinition, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}

	t := &TriggerDefinition{ConfigChange: true}
	switch typed := obj.(type) {
	case *extensionsv1beta1.Deployment:
		t.ConfigChange = !typed.Spec.Paused
	case *kappsv1beta1.Deployment:
		t.ConfigChange = !typed.Spec.Paused
	case *kappsv1beta2.Deployment:
		t.ConfigChange = !typed.Spec.Paused
	case *kappsv1.Deployment:
		t.ConfigChange = !typed.Spec.Paused
	}

	out, ok := m.GetAnnotations()[triggerapi.TriggerAnnotationKey]
	if !ok {
		return t, nil
	}
	triggers := []triggerapi.ObjectFieldTrigger{}
	if err := json.Unmarshal([]byte(out), &triggers); err != nil {
		return nil, err
	}

	for _, trigger := range triggers {
		container, remainder, err := annotations.ContainerForObjectFieldPath(obj, trigger.FieldPath)
		if err != nil || remainder != "image" {
			continue
		}
		t.ImageChange = append(t.ImageChange, ImageChangeTrigger{
			Auto:      !trigger.Paused,
			Names:     []string{container.GetName()},
			From:      trigger.From.Name,
			Namespace: defaultNamespace(trigger.From.Namespace, m.GetNamespace()),
		})
	}
	return t, nil
}

// NewDeploymentConfigTriggers creates a trigger definition from a deployment config.
func NewDeploymentConfigTriggers(config *appsv1.DeploymentConfig) *TriggerDefinition {
	t := &TriggerDefinition{}
	for _, trigger := range config.Spec.Triggers {
		switch trigger.Type {
		case appsv1.DeploymentTriggerOnConfigChange:
			t.ConfigChange = true
		case appsv1.DeploymentTriggerOnImageChange:
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
func NewBuildConfigTriggers(config *buildv1.BuildConfig) *TriggerDefinition {
	t := &TriggerDefinition{}
	setStrategy := false
	for _, trigger := range config.Spec.Triggers {
		switch trigger.Type {
		case buildv1.ConfigChangeBuildTriggerType:
			t.ConfigChange = true
		case buildv1.GenericWebHookBuildTriggerType:
			t.GenericWebHooks = append(t.GenericWebHooks,
				buildv1.WebHookTrigger{
					Secret:          trigger.GenericWebHook.Secret,
					SecretReference: trigger.GenericWebHook.SecretReference,
					AllowEnv:        trigger.GenericWebHook.AllowEnv,
				},
			)
		case buildv1.GitHubWebHookBuildTriggerType:
			t.GitHubWebHooks = append(t.GitHubWebHooks,
				buildv1.WebHookTrigger{
					Secret:          trigger.GitHubWebHook.Secret,
					SecretReference: trigger.GitHubWebHook.SecretReference,
				},
			)
		case buildv1.GitLabWebHookBuildTriggerType:
			t.GitLabWebHooks = append(t.GitLabWebHooks,
				buildv1.WebHookTrigger{
					Secret:          trigger.GitLabWebHook.Secret,
					SecretReference: trigger.GitLabWebHook.SecretReference,
				},
			)
		case buildv1.BitbucketWebHookBuildTriggerType:
			t.BitbucketWebHooks = append(t.BitbucketWebHooks,
				buildv1.WebHookTrigger{
					Secret:          trigger.BitbucketWebHook.Secret,
					SecretReference: trigger.BitbucketWebHook.SecretReference,
				},
			)
		case buildv1.ImageChangeBuildTriggerType:
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

// Apply writes a trigger definition back to an object.
func (t *TriggerDefinition) Apply(obj runtime.Object) error {
	switch c := obj.(type) {
	case *appsv1.DeploymentConfig:
		if len(t.GitHubWebHooks) > 0 {
			return fmt.Errorf("deployment configs do not support GitHub web hooks")
		}
		if len(t.GenericWebHooks) > 0 {
			return fmt.Errorf("deployment configs do not support web hooks")
		}
		if len(t.GitLabWebHooks) > 0 {
			return fmt.Errorf("deployment configs do not support GitLab web hooks")
		}
		if len(t.BitbucketWebHooks) > 0 {
			return fmt.Errorf("deployment configs do not support Bitbucket web hooks")
		}

		existingTriggers := filterDeploymentTriggers(c.Spec.Triggers, appsv1.DeploymentTriggerOnConfigChange)
		var triggers []appsv1.DeploymentTriggerPolicy
		if t.ConfigChange {
			triggers = append(triggers, appsv1.DeploymentTriggerPolicy{Type: appsv1.DeploymentTriggerOnConfigChange})
		}
		allNames := sets.NewString()
		for _, container := range c.Spec.Template.Spec.Containers {
			allNames.Insert(container.Name)
		}
		for _, container := range c.Spec.Template.Spec.InitContainers {
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
			triggers = append(triggers, appsv1.DeploymentTriggerPolicy{
				Type: appsv1.DeploymentTriggerOnImageChange,
				ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
					Automatic: trigger.Auto,
					From: corev1.ObjectReference{
						Kind:      "ImageStreamTag",
						Name:      trigger.From,
						Namespace: trigger.Namespace,
					},
					ContainerNames: trigger.Names,
				},
			})
		}
		c.Spec.Triggers = mergeDeployTriggers(existingTriggers, triggers)
		return nil

	case *buildv1.BuildConfig:
		var triggers []buildv1.BuildTriggerPolicy
		if t.ConfigChange {
			triggers = append(triggers, buildv1.BuildTriggerPolicy{Type: buildv1.ConfigChangeBuildTriggerType})
		}
		for i := range t.GenericWebHooks {
			triggers = append(triggers, buildv1.BuildTriggerPolicy{
				Type:           buildv1.GenericWebHookBuildTriggerType,
				GenericWebHook: &t.GenericWebHooks[i],
			})
		}
		for i := range t.GitHubWebHooks {
			triggers = append(triggers, buildv1.BuildTriggerPolicy{
				Type:          buildv1.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &t.GitHubWebHooks[i],
			})
		}
		for i := range t.GitLabWebHooks {
			triggers = append(triggers, buildv1.BuildTriggerPolicy{
				Type:          buildv1.GitLabWebHookBuildTriggerType,
				GitLabWebHook: &t.GitLabWebHooks[i],
			})
		}
		for i := range t.BitbucketWebHooks {
			triggers = append(triggers, buildv1.BuildTriggerPolicy{
				Type:             buildv1.BitbucketWebHookBuildTriggerType,
				BitbucketWebHook: &t.BitbucketWebHooks[i],
			})
		}

		// add new triggers, filter out any old triggers that match (if moving from automatic to manual),
		// and then merge the old triggers and the new triggers to preserve fields like lastTriggeredImageID
		existingTriggers := c.Spec.Triggers
		strategyTrigger := strategyTrigger(c)
		for _, trigger := range t.ImageChange {
			change := &buildv1.ImageChangeTrigger{
				From: &corev1.ObjectReference{
					Kind:      "ImageStreamTag",
					Name:      trigger.From,
					Namespace: trigger.Namespace,
				},
			}

			// use the canonical ImageChangeTrigger with nil From
			if strategyTrigger != nil {
				strategyTrigger.Auto = trigger.Auto
			}
			if reflect.DeepEqual(strategyTrigger, &trigger) {
				change.From = nil
			}

			// if this trigger is not automatic, then we need to remove it from the list of triggers
			if !trigger.Auto {
				existingTriggers = filterBuildImageTriggers(existingTriggers, trigger, strategyTrigger)
				continue
			}

			triggers = append(triggers, buildv1.BuildTriggerPolicy{
				Type:        buildv1.ImageChangeBuildTriggerType,
				ImageChange: change,
			})
		}
		c.Spec.Triggers = mergeBuildTriggers(existingTriggers, triggers)
		return nil

	case *extensionsv1beta1.DaemonSet, *kappsv1beta2.DaemonSet, *kappsv1.DaemonSet,
		*extensionsv1beta1.Deployment, *kappsv1beta1.Deployment, *kappsv1beta2.Deployment, *kappsv1.Deployment,
		*kappsv1beta1.StatefulSet, *kappsv1beta2.StatefulSet, *kappsv1.StatefulSet,
		*batchv1beta1.CronJob, *batchv2alpha1.CronJob:

		if len(t.GitHubWebHooks) > 0 {
			return fmt.Errorf("does not support GitHub web hooks")
		}
		if len(t.GenericWebHooks) > 0 {
			return fmt.Errorf("does not support web hooks")
		}
		if len(t.GitLabWebHooks) > 0 {
			return fmt.Errorf("does not support GitLab web hooks")
		}
		if len(t.BitbucketWebHooks) > 0 {
			return fmt.Errorf("does not support Bitbucket web hooks")
		}
		m, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		spec, path, err := ometa.GetPodSpecV1(obj)
		if err != nil {
			return err
		}
		allNames := sets.NewString()
		for _, container := range spec.Containers {
			allNames.Insert(container.Name)
		}
		for _, container := range spec.InitContainers {
			allNames.Insert(container.Name)
		}
		alreadyTriggered := sets.NewString()
		var triggers []triggerapi.ObjectFieldTrigger
		glog.V(4).Infof("calculated triggers: %#v", t.ImageChange)
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
			if alreadyTriggered.HasAny(trigger.Names...) {
				return fmt.Errorf("only one trigger may reference each container: %s", strings.Join(alreadyTriggered.Intersection(sets.NewString(trigger.Names...)).List(), ", "))
			}
			alreadyTriggered.Insert(trigger.Names...)

			ns := trigger.Namespace
			if ns == m.GetNamespace() {
				ns = ""
			}
			for _, name := range trigger.Names {
				triggers = append(triggers, triggerapi.ObjectFieldTrigger{
					From: triggerapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Name:      trigger.From,
						Namespace: ns,
					},
					FieldPath: fmt.Sprintf(path.Child("containers").String()+"[?(@.name==\"%s\")].image", name),
					Paused:    !trigger.Auto,
				})
			}
		}
		out, err := json.Marshal(triggers)
		if err != nil {
			return err
		}
		a := m.GetAnnotations()
		if a == nil {
			a = make(map[string]string)
		}
		a[triggerapi.TriggerAnnotationKey] = string(out)
		m.SetAnnotations(a)

		switch typed := obj.(type) {
		case *extensionsv1beta1.Deployment:
			typed.Spec.Paused = !t.ConfigChange
		case *kappsv1beta1.Deployment:
			typed.Spec.Paused = !t.ConfigChange
		case *kappsv1beta2.Deployment:
			typed.Spec.Paused = !t.ConfigChange
		case *kappsv1.Deployment:
			typed.Spec.Paused = !t.ConfigChange
		}
		glog.V(4).Infof("Updated annotated object: %#v", obj)
		return nil

	default:
		return fmt.Errorf("the object is not a deployment config or build config")
	}
}

// triggerMatchesBuildImageChange identifies whether the image change is equivalent to the trigger
func triggerMatchesBuildImageChange(trigger ImageChangeTrigger, strategyTrigger *ImageChangeTrigger, imageChange *buildv1.ImageChangeTrigger) bool {
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
func filterBuildImageTriggers(src []buildv1.BuildTriggerPolicy, trigger ImageChangeTrigger, strategyTrigger *ImageChangeTrigger) []buildv1.BuildTriggerPolicy {
	var dst []buildv1.BuildTriggerPolicy
	for i := range src {
		if triggerMatchesBuildImageChange(trigger, strategyTrigger, src[i].ImageChange) {
			continue
		}
		dst = append(dst, src[i])
	}
	return dst
}

// filterDeploymentTriggers returns only triggers that do not have one of the provided types.
func filterDeploymentTriggers(src []appsv1.DeploymentTriggerPolicy, types ...appsv1.DeploymentTriggerType) []appsv1.DeploymentTriggerPolicy {
	var dst []appsv1.DeploymentTriggerPolicy
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
func strategyTrigger(config *buildv1.BuildConfig) *ImageChangeTrigger {
	if from := getInputReference(config.Spec.Strategy); from != nil {
		if from.Kind == "ImageStreamTag" {
			// normalize the strategy object reference
			from.Namespace = defaultNamespace(from.Namespace, config.Namespace)
			return &ImageChangeTrigger{From: from.Name, Namespace: from.Namespace}
		}
	}
	return nil
}

// mergeDeployTriggers returns an array of DeploymentTriggerPolicies that have no duplicates.
func mergeDeployTriggers(dst, src []appsv1.DeploymentTriggerPolicy) []appsv1.DeploymentTriggerPolicy {
	// never return an empty map, because the triggers on a deployment config default when the map is empty
	result := []appsv1.DeploymentTriggerPolicy{}
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
func findDeployTrigger(dst []appsv1.DeploymentTriggerPolicy, trigger appsv1.DeploymentTriggerPolicy) int {
	for i := range dst {
		if reflect.DeepEqual(dst[i], trigger) {
			return i
		}
	}
	return -1
}

// mergeBuildTriggers returns an array of BuildTriggerPolicies that have no duplicates, in the same order
// as they exist in their original arrays (a zip-merge).
func mergeBuildTriggers(dst, src []buildv1.BuildTriggerPolicy) []buildv1.BuildTriggerPolicy {
	var result []buildv1.BuildTriggerPolicy
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
func findBuildTrigger(dst []buildv1.BuildTriggerPolicy, trigger buildv1.BuildTriggerPolicy) int {
	// make a copy for semantic equality
	if trigger.ImageChange != nil {
		trigger.ImageChange = &buildv1.ImageChangeTrigger{From: trigger.ImageChange.From}
	}
	for i, copied := range dst {
		// make a copy for semantic equality
		if copied.ImageChange != nil {
			copied.ImageChange = &buildv1.ImageChangeTrigger{From: copied.ImageChange.From}
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
	case *appsv1.DeploymentConfig:
		triggers := NewDeploymentConfigTriggers(t)
		if err := fn(triggers); err != nil {
			return true, err
		}
		return true, triggers.Apply(t)
	case *buildv1.BuildConfig:
		triggers := NewBuildConfigTriggers(t)
		if err := fn(triggers); err != nil {
			return true, err
		}
		return true, triggers.Apply(t)
	case *extensionsv1beta1.DaemonSet, *kappsv1beta2.DaemonSet, *kappsv1.DaemonSet,
		*extensionsv1beta1.Deployment, *kappsv1beta1.Deployment, *kappsv1beta2.Deployment, *kappsv1.Deployment,
		*kappsv1beta1.StatefulSet, *kappsv1beta2.StatefulSet, *kappsv1.StatefulSet,
		*batchv1beta1.CronJob, *batchv2alpha1.CronJob:
		triggers, err := NewAnnotationTriggers(obj)
		if err != nil {
			return false, err
		}
		if err := fn(triggers); err != nil {
			return true, err
		}
		return true, triggers.Apply(obj)
	default:
		return false, fmt.Errorf("the object does not support triggers: %T", t)
	}
}

func getInputReference(strategy buildv1.BuildStrategy) *corev1.ObjectReference {
	switch {
	case strategy.SourceStrategy != nil:
		return &strategy.SourceStrategy.From
	case strategy.DockerStrategy != nil:
		return strategy.DockerStrategy.From
	case strategy.CustomStrategy != nil:
		return &strategy.CustomStrategy.From
	default:
		return nil
	}
}
