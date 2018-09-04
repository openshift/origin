package set

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kinternalclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	buildv1 "github.com/openshift/api/build/v1"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/util/clientcmd"
	utilenv "github.com/openshift/origin/pkg/oc/util/env"
	envresolve "github.com/openshift/origin/pkg/pod/envresolve/internal_version"
)

var (
	envLong = templates.LongDesc(`
		Update environment variables on a pod template or a build config

		List environment variable definitions in one or more pods, pod templates or build
		configuration.
		Add, update, or remove container environment variable definitions in one or
		more pod templates (within replication controllers or deployment configurations) or
		build configurations.
		View or modify the environment variable definitions on all containers in the
		specified pods or pod templates, or just those that match a wildcard.

		If "--env -" is passed, environment variables can be read from STDIN using the standard env
		syntax.`)

	envExample = templates.Examples(`
		# Update deployment 'registry' with a new environment variable
	  %[1]s env dc/registry STORAGE_DIR=/local

	  # List the environment variables defined on a build config 'sample-build'
	  %[1]s env bc/sample-build --list

	  # List the environment variables defined on all pods
	  %[1]s env pods --all --list

	  # Output modified build config in YAML
	  %[1]s env bc/sample-build STORAGE_DIR=/data -o yaml

	  # Update all containers in all replication controllers in the project to have ENV=prod
	  %[1]s env rc --all ENV=prod

	  # Import environment from a secret
	  %[1]s env --from=secret/mysecret dc/myapp

	  # Import environment from a config map with a prefix
	  %[1]s env --from=configmap/myconfigmap --prefix=MYSQL_ dc/myapp

	  # Remove the environment variable ENV from container 'c1' in all deployment configs
	  %[1]s env dc --all --containers="c1" ENV-

	  # Remove the environment variable ENV from a deployment config definition on disk and
	  # update the deployment config on the server
	  %[1]s env -f dc.json ENV-

	  # Set some of the local shell environment into a deployment config on the server
	  env | grep RAILS_ | %[1]s env -e - dc/registry`)
)

type EnvOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	EnvParams []string
	EnvArgs   []string
	Resources []string

	All       bool
	Resolve   bool
	List      bool
	Local     bool
	Overwrite bool
	DryRun    bool

	ResourceVersion   string
	ContainerSelector string
	Selector          string
	From              string
	Prefix            string

	UpdatePodSpecForObject polymorphichelpers.UpdatePodSpecForObjectFunc
	Builder                func() *resource.Builder
	Encoder                runtime.Encoder
	Mapper                 meta.RESTMapper
	Client                 dynamic.Interface
	KubeClient             kinternalclientset.Interface
	Printer                printers.ResourcePrinter
	Namespace              string
	ExplicitNamespace      bool

	genericclioptions.IOStreams
	resource.FilenameOptions
}

func NewEnvOptions(streams genericclioptions.IOStreams) *EnvOptions {
	return &EnvOptions{
		PrintFlags: genericclioptions.NewPrintFlags("updated").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,

		ContainerSelector: "*",
		Overwrite:         true,
	}
}

// NewCmdEnv implements the OpenShift cli env command
func NewCmdEnv(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewEnvOptions(streams)
	cmd := &cobra.Command{
		Use:     "env RESOURCE/NAME KEY_1=VAL_1 ... KEY_N=VAL_N",
		Short:   "Update environment variables on a pod template",
		Long:    envLong,
		Example: fmt.Sprintf(envExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.RunEnv())
		},
	}
	usage := "to use to edit the resource"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.ContainerSelector, "containers", "c", o.ContainerSelector, "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().StringVar(&o.From, "from", o.From, "The name of a resource from which to inject environment variables")
	cmd.Flags().StringVar(&o.Prefix, "prefix", o.Prefix, "Prefix to append to variable names")
	cmd.Flags().StringArrayVarP(&o.EnvParams, "env", "e", o.EnvParams, "Specify a key-value pair for an environment variable to set into each container.")
	cmd.Flags().BoolVar(&o.List, "list", o.List, "If true, display the environment and any changes in the standard format")
	cmd.Flags().BoolVar(&o.Resolve, "resolve", o.Resolve, "If true, show secret or configmap references when listing variables")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set image will NOT contact api-server but run locally.")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, select all resources in the namespace of the specified resource types")
	cmd.Flags().BoolVar(&o.Overwrite, "overwrite", o.Overwrite, "If true, allow environment to be overwritten, otherwise reject updates that overwrite existing environment.")
	cmd.Flags().StringVar(&o.ResourceVersion, "resource-version", o.ResourceVersion, "If non-empty, the labels update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.")

	kcmdutil.AddDryRunFlag(cmd)
	o.PrintFlags.AddFlags(cmd)

	return cmd
}

func validateNoOverwrites(existing []kapi.EnvVar, env []kapi.EnvVar) error {
	for _, e := range env {
		if current, exists := findEnv(existing, e.Name); exists && current.Value != e.Value {
			return fmt.Errorf("'%s' already has a value (%s), and --overwrite is false", current.Name, current.Value)
		}
	}
	return nil
}

func keyToEnvName(key string) string {
	validEnvNameRegexp := regexp.MustCompile("[^a-zA-Z0-9_]")
	return strings.ToUpper(validEnvNameRegexp.ReplaceAllString(key, "_"))
}

func (o *EnvOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var ok bool
	o.Resources, o.EnvArgs, ok = utilenv.SplitEnvironmentFromResources(args)
	if !ok {
		return kcmdutil.UsageErrorf(cmd, "all resources must be specified before environment changes: %s", strings.Join(args, " "))
	}
	if len(o.Filenames) == 0 && len(o.Resources) < 1 {
		return kcmdutil.UsageErrorf(cmd, "one or more resources must be specified as <resource> <name> or <resource>/<name>")
	}

	var err error
	o.KubeClient, err = f.ClientSet()
	if err != nil {
		return err
	}

	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.Builder = f.NewBuilder
	o.UpdatePodSpecForObject = polymorphichelpers.UpdatePodSpecForObjectFn

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	if o.List && o.PrintFlags.OutputFormat != nil && len(*o.PrintFlags.OutputFormat) > 0 {
		return kcmdutil.UsageErrorf(cmd, "--list and --output may not be specified together")
	}

	cmdutil.WarnAboutCommaSeparation(o.ErrOut, o.EnvParams, "--env")

	return nil
}

// RunEnv contains all the necessary functionality for the OpenShift cli env command
// TODO: refactor to share the common "patch resource" pattern of probe
// TODO: figure out how we can replace this with upstream counterpart
func (o *EnvOptions) RunEnv() error {
	env, remove, err := utilenv.ParseEnv(append(o.EnvParams, o.EnvArgs...), o.In)
	if err != nil {
		return err
	}

	if len(o.From) != 0 {
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
				ResourceTypeOrNameArgs(o.All, o.From).
				Latest()
		}

		singleItemImplied := false
		infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
		if err != nil {
			return err
		}

		for _, info := range infos {
			switch from := info.Object.(type) {
			case *corev1.Secret:
				for key := range from.Data {
					envVar := kapi.EnvVar{
						Name: keyToEnvName(key),
						ValueFrom: &kapi.EnvVarSource{
							SecretKeyRef: &kapi.SecretKeySelector{
								LocalObjectReference: kapi.LocalObjectReference{
									Name: from.Name,
								},
								Key: key,
							},
						},
					}
					env = append(env, envVar)
				}
			case *corev1.ConfigMap:
				for key := range from.Data {
					envVar := kapi.EnvVar{
						Name: keyToEnvName(key),
						ValueFrom: &kapi.EnvVarSource{
							ConfigMapKeyRef: &kapi.ConfigMapKeySelector{
								LocalObjectReference: kapi.LocalObjectReference{
									Name: from.Name,
								},
								Key: key,
							},
						},
					}
					env = append(env, envVar)
				}
			default:
				return fmt.Errorf("unsupported resource specified in --from: %T", from)
			}
		}
	}

	if len(o.Prefix) != 0 {
		for i := range env {
			env[i].Name = fmt.Sprintf("%s%s", o.Prefix, env[i].Name)
		}
	}

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
			ResourceTypeOrNameArgs(o.All, o.Resources...).
			Latest()
	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	// only apply resource version locking on a single resource
	if !singleItemImplied && len(o.ResourceVersion) > 0 {
		return fmt.Errorf("--resource-version may only be used with a single resource")
	}
	// Keep a copy of the original objects prior to updating their environment.
	// Used in constructing the patch(es) that will be applied in the server.
	oldObjects := []runtime.Object{}
	oldData := make([][]byte, len(infos))
	for i := range infos {
		oldObjects = append(oldObjects, kcmdutil.AsDefaultVersionedOrOriginal(infos[i].Object.DeepCopyObject(), nil))
		old, err := json.Marshal(oldObjects[i])
		if err != nil {
			return err
		}
		oldData[i] = old
	}

	skipped := 0
	errored := []*resource.Info{}
	for _, info := range infos {
		name := getObjectName(info)
		ok, err := o.UpdatePodSpecForObject(info.Object, clientcmd.ConvertInteralPodSpecToExternal(func(spec *kapi.PodSpec) error {
			resolutionErrorsEncountered := false
			containers, _ := selectContainers(spec.Containers, o.ContainerSelector)
			if len(containers) == 0 {
				fmt.Fprintf(o.ErrOut, "warning: %s does not have any containers matching %q\n", name, o.ContainerSelector)
				return nil
			}
			for _, c := range containers {
				if !o.Overwrite {
					if err := validateNoOverwrites(c.Env, env); err != nil {
						errored = append(errored, info)
						return err
					}
				}

				c.Env = updateEnv(c.Env, env, remove)

				if o.List {
					resolveErrors := map[string][]string{}
					store := envresolve.NewResourceStore()

					fmt.Fprintf(o.Out, "# %s, container %s\n", name, c.Name)
					for _, env := range c.Env {
						// Print the simple value
						if env.ValueFrom == nil {
							fmt.Fprintf(o.Out, "%s=%s\n", env.Name, env.Value)
							continue
						}

						// Print the reference version
						if !o.Resolve {
							fmt.Fprintf(o.Out, "# %s from %s\n", env.Name, envresolve.GetEnvVarRefString(env.ValueFrom))
							continue
						}

						value, err := envresolve.GetEnvVarRefValue(o.KubeClient, o.Namespace, store, env.ValueFrom, info.Object, c)
						// Print the resolved value
						if err == nil {
							fmt.Fprintf(o.Out, "%s=%s\n", env.Name, value)
							continue
						}

						// Print the reference version and save the resolve error
						fmt.Fprintf(o.Out, "# %s from %s\n", env.Name, envresolve.GetEnvVarRefString(env.ValueFrom))
						errString := err.Error()
						resolveErrors[errString] = append(resolveErrors[errString], env.Name)
						resolutionErrorsEncountered = true
					}

					// Print any resolution errors
					errs := []string{}
					for err, vars := range resolveErrors {
						sort.Strings(vars)
						errs = append(errs, fmt.Sprintf("error retrieving reference for %s: %v", strings.Join(vars, ", "), err))
					}
					sort.Strings(errs)
					for _, err := range errs {
						fmt.Fprintln(o.ErrOut, err)
					}
				}
			}
			if resolutionErrorsEncountered {
				errored = append(errored, info)
				return errors.New("failed to retrieve valueFrom references")
			}
			return nil
		}))
		if !ok {
			// This is a fallback function for objects that don't have pod spec.
			ok, err = updateObjectEnvironment(info.Object, func(vars *[]kapi.EnvVar) error {
				if vars == nil {
					return fmt.Errorf("no environment variables provided")
				}
				if !o.Overwrite {
					if err := validateNoOverwrites(*vars, env); err != nil {
						errored = append(errored, info)
						return err
					}
				}
				*vars = updateEnv(*vars, env, remove)
				if o.List {
					fmt.Fprintf(o.Out, "# %s\n", name)
					for _, env := range *vars {
						fmt.Fprintf(o.Out, "%s=%s\n", env.Name, env.Value)
					}
				}
				return nil
			})
			if !ok {
				skipped++
				continue
			}
		}
		if err != nil {
			fmt.Fprintf(o.ErrOut, "error: %s %v\n", name, err)
			continue
		}
	}
	if singleItemImplied && skipped == len(infos) {
		name := getObjectName(infos[0])
		return fmt.Errorf("%s is not a pod or does not have a pod template", name)
	}
	if len(errored) == len(infos) {
		return kcmdutil.ErrExit
	}

	if o.List {
		return nil
	}

	allErrs := []error{}
updates:
	for i, info := range infos {
		for _, erroredInfo := range errored {
			if info == erroredInfo {
				continue updates
			}
		}

		if o.Local || o.DryRun {
			if err := o.Printer.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		newData, err := json.Marshal(kcmdutil.AsDefaultVersionedOrOriginal(infos[i].Object, nil))
		if err != nil {
			allErrs = append(allErrs, err)
			continue
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData[i], newData, kcmdutil.AsDefaultVersionedOrOriginal(infos[i].Object, nil))
		if err != nil {
			allErrs = append(allErrs, err)
			continue
		}

		actual, err := o.Client.Resource(info.Mapping.Resource).Namespace(info.Namespace).Patch(info.Name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to set env: %v\n", err))
			continue
		}

		// make sure arguments to set or replace environment variables are set
		// before returning a successful message
		if len(env) == 0 && len(o.EnvArgs) == 0 && len(remove) == 0 {
			return fmt.Errorf("at least one environment variable must be provided")
		}

		if err := o.Printer.PrintObj(actual, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}

// UpdateObjectEnvironment update the environment variables in object specification.
func updateObjectEnvironment(obj runtime.Object, fn func(*[]kapi.EnvVar) error) (bool, error) {
	switch t := obj.(type) {
	case *buildv1.BuildConfig:
		if t.Spec.Strategy.CustomStrategy != nil {
			return true, convertInternalEnvVarToExternal(fn)(&t.Spec.Strategy.CustomStrategy.Env)
		}
		if t.Spec.Strategy.SourceStrategy != nil {
			return true, convertInternalEnvVarToExternal(fn)(&t.Spec.Strategy.SourceStrategy.Env)
		}
		if t.Spec.Strategy.DockerStrategy != nil {
			return true, convertInternalEnvVarToExternal(fn)(&t.Spec.Strategy.DockerStrategy.Env)
		}
		if t.Spec.Strategy.JenkinsPipelineStrategy != nil {
			return true, convertInternalEnvVarToExternal(fn)(&t.Spec.Strategy.JenkinsPipelineStrategy.Env)
		}
	}
	return false, fmt.Errorf("object does not contain any environment variables %T", obj)
}

// TODO: this needs to die when switching to external versions
func convertInternalEnvVarToExternal(inFn func(*[]kapi.EnvVar) error) func(*[]corev1.EnvVar) error {
	return func(specToMutate *[]corev1.EnvVar) error {
		externalEnvVar := &[]kapi.EnvVar{}
		if err := legacyscheme.Scheme.Convert(specToMutate, externalEnvVar, nil); err != nil {
			return err
		}
		if err := inFn(externalEnvVar); err != nil {
			return err
		}
		internalEnvVar := &[]corev1.EnvVar{}
		if err := legacyscheme.Scheme.Convert(externalEnvVar, internalEnvVar, nil); err != nil {
			return err
		}
		*specToMutate = *internalEnvVar
		return nil
	}
}
