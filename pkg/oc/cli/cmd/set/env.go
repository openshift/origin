package set

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	utilenv "github.com/openshift/origin/pkg/oc/util/env"
	envresolve "github.com/openshift/origin/pkg/pod/envresolve"
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

	  # Output modified build config in YAML, and does not alter the object on the server
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
	Out io.Writer
	Err io.Writer
	In  io.Reader

	Filenames []string
	EnvParams []string
	EnvArgs   []string
	Resources []string

	All         bool
	Resolve     bool
	List        bool
	ShortOutput bool
	Local       bool
	Overwrite   bool

	ResourceVersion   string
	ContainerSelector string
	Selector          string
	Output            string
	From              string
	Prefix            string

	Builder *resource.Builder
	Infos   []*resource.Info

	Encoder runtime.Encoder

	Cmd *cobra.Command

	Mapper meta.RESTMapper

	PrintObject func([]*resource.Info) error
}

// NewCmdEnv implements the OpenShift cli env command
func NewCmdEnv(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &EnvOptions{
		Out: out,
		Err: errout,
		In:  in,
	}
	cmd := &cobra.Command{
		Use:     "env RESOURCE/NAME KEY_1=VAL_1 ... KEY_N=VAL_N",
		Short:   "Update environment variables on a pod template",
		Long:    envLong,
		Example: fmt.Sprintf(envExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			err := options.RunEnv(f)
			if err == kcmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&options.ContainerSelector, "containers", "c", "*", "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().StringP("from", "", "", "The name of a resource from which to inject environment variables")
	cmd.Flags().StringP("prefix", "", "", "Prefix to append to variable names")
	cmd.Flags().StringArrayVarP(&options.EnvParams, "env", "e", options.EnvParams, "Specify a key-value pair for an environment variable to set into each container.")
	cmd.Flags().BoolVar(&options.List, "list", options.List, "If true, display the environment and any changes in the standard format")
	cmd.Flags().BoolVar(&options.Resolve, "resolve", options.Resolve, "If true, show secret or configmap references when listing variables")
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&options.Local, "local", false, "If true, set image will NOT contact api-server but run locally.")
	cmd.Flags().BoolVar(&options.All, "all", options.All, "If true, select all resources in the namespace of the specified resource types")
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")
	cmd.Flags().BoolVar(&options.Overwrite, "overwrite", true, "If true, allow environment to be overwritten, otherwise reject updates that overwrite existing environment.")
	cmd.Flags().String("resource-version", "", "If non-empty, the labels update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.")

	kcmdutil.AddDryRunFlag(cmd)
	kcmdutil.AddPrinterFlags(cmd)

	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")

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

func (o *EnvOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	resources, envArgs, ok := utilenv.SplitEnvironmentFromResources(args)
	if !ok {
		return kcmdutil.UsageErrorf(cmd, "all resources must be specified before environment changes: %s", strings.Join(args, " "))
	}
	if len(o.Filenames) == 0 && len(resources) < 1 {
		return kcmdutil.UsageErrorf(cmd, "one or more resources must be specified as <resource> <name> or <resource>/<name>")
	}

	o.ContainerSelector = kcmdutil.GetFlagString(cmd, "containers")
	o.List = kcmdutil.GetFlagBool(cmd, "list")
	o.Resolve = kcmdutil.GetFlagBool(cmd, "resolve")
	o.Selector = kcmdutil.GetFlagString(cmd, "selector")
	o.All = kcmdutil.GetFlagBool(cmd, "all")
	o.Overwrite = kcmdutil.GetFlagBool(cmd, "overwrite")
	o.ResourceVersion = kcmdutil.GetFlagString(cmd, "resource-version")
	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.From = kcmdutil.GetFlagString(cmd, "from")
	o.Prefix = kcmdutil.GetFlagString(cmd, "prefix")

	o.EnvArgs = envArgs
	o.Resources = resources
	o.Cmd = cmd

	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"

	if o.List && len(o.Output) > 0 {
		return kcmdutil.UsageErrorf(o.Cmd, "--list and --output may not be specified together")
	}

	return nil
}

// RunEnv contains all the necessary functionality for the OpenShift cli env command
// TODO: refactor to share the common "patch resource" pattern of probe
func (o *EnvOptions) RunEnv(f *clientcmd.Factory) error {
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	kubeClient, err := f.ClientSet()
	if err != nil {
		return err
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	cmdutil.WarnAboutCommaSeparation(o.Err, o.EnvParams, "--env")

	env, remove, err := utilenv.ParseEnv(append(o.EnvParams, o.EnvArgs...), o.In)
	if err != nil {
		return err
	}

	if len(o.From) != 0 {
		b := f.NewBuilder().
			Internal().
			LocalParam(o.Local).
			ContinueOnError().
			NamespaceParam(cmdNamespace).DefaultNamespace().
			FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
			Flatten()

		if !o.Local {
			b = b.
				LabelSelectorParam(o.Selector).
				ResourceTypeOrNameArgs(o.All, o.From)
		}

		one := false
		infos, err := b.Do().IntoSingleItemImplied(&one).Infos()
		if err != nil {
			return err
		}

		for _, info := range infos {
			switch from := info.Object.(type) {
			case *kapi.Secret:
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
			case *kapi.ConfigMap:
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
				return fmt.Errorf("unsupported resource specified in --from")
			}
		}
	}

	if len(o.Prefix) != 0 {
		for i := range env {
			env[i].Name = fmt.Sprintf("%s%s", o.Prefix, env[i].Name)
		}
	}

	b := f.NewBuilder().
		Internal().
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		Flatten()

	if !o.Local {
		b = b.
			LabelSelectorParam(o.Selector).
			ResourceTypeOrNameArgs(o.All, o.Resources...)
	}

	one := false
	infos, err := b.Do().IntoSingleItemImplied(&one).Infos()
	if err != nil {
		return err
	}

	// only apply resource version locking on a single resource
	if !one && len(o.ResourceVersion) > 0 {
		return kcmdutil.UsageErrorf(o.Cmd, "--resource-version may only be used with a single resource")
	}
	// Keep a copy of the original objects prior to updating their environment.
	// Used in constructing the patch(es) that will be applied in the server.
	gv := *clientConfig.GroupVersion
	oldObjects, err := clientcmd.AsVersionedObjects(infos, gv, legacyscheme.Codecs.LegacyCodec(gv))
	if err != nil {
		return err
	}
	if len(oldObjects) != len(infos) {
		return fmt.Errorf("could not convert all objects to API version %q", clientConfig.GroupVersion)
	}
	oldData := make([][]byte, len(infos))
	for i := range oldObjects {
		old, err := json.Marshal(oldObjects[i])
		if err != nil {
			return err
		}
		oldData[i] = old
	}

	skipped := 0
	errored := []*resource.Info{}
	for _, info := range infos {
		ok, err := f.UpdatePodSpecForObject(info.Object, clientcmd.ConvertInteralPodSpecToExternal(func(spec *kapi.PodSpec) error {
			resolutionErrorsEncountered := false
			containers, _ := selectContainers(spec.Containers, o.ContainerSelector)
			if len(containers) == 0 {
				fmt.Fprintf(o.Err, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource, info.Name, o.ContainerSelector)
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

					fmt.Fprintf(o.Out, "# %s %s, container %s\n", info.Mapping.Resource, info.Name, c.Name)
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

						value, err := envresolve.GetEnvVarRefValue(kubeClient, cmdNamespace, store, env.ValueFrom, info.Object, c)
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
						fmt.Fprintln(o.Err, err)
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
			ok, err = f.UpdateObjectEnvironment(info.Object, func(vars *[]kapi.EnvVar) error {
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
					fmt.Fprintf(o.Out, "# %s %s\n", info.Mapping.Resource, info.Name)
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
			fmt.Fprintf(o.Err, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, err)
			continue
		}
	}
	if one && skipped == len(infos) {
		return fmt.Errorf("%s/%s is not a pod or does not have a pod template", infos[0].Mapping.Resource, infos[0].Name)
	}
	if len(errored) == len(infos) {
		return kcmdutil.ErrExit
	}

	if o.List {
		return nil
	}

	if len(o.Output) > 0 || o.Local || kcmdutil.GetDryRunFlag(o.Cmd) {
		return f.PrintResourceInfos(o.Cmd, o.Local, infos, o.Out)
	}

	objects, err := clientcmd.AsVersionedObjects(infos, gv, legacyscheme.Codecs.LegacyCodec(gv))
	if err != nil {
		return err
	}
	if len(objects) != len(infos) {
		return fmt.Errorf("could not convert all objects to API version %q", clientConfig.GroupVersion)
	}

	failed := false
updates:
	for i, info := range infos {
		for _, erroredInfo := range errored {
			if info == erroredInfo {
				continue updates
			}
		}
		newData, err := json.Marshal(objects[i])
		if err != nil {
			return err
		}
		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData[i], newData, objects[i])
		if err != nil {
			return err
		}
		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patchBytes)
		if err != nil {
			handlePodUpdateError(o.Err, err, "environment variables")
			failed = true
			continue
		}
		info.Refresh(obj, true)

		// make sure arguments to set or replace environment variables are set
		// before returning a successful message
		if len(env) == 0 && len(o.EnvArgs) == 0 && len(remove) == 0 {
			return fmt.Errorf("at least one environment variable must be provided")
		}

		mapper, _ := f.Object()
		kcmdutil.PrintSuccess(mapper, o.ShortOutput, o.Out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if failed {
		return kcmdutil.ErrExit
	}
	return nil
}
