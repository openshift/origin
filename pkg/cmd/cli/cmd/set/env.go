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
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fieldpath"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/strategicpatch"

	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
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

// NewCmdEnv implements the OpenShift cli env command
func NewCmdEnv(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	var filenames []string
	var env []string
	cmd := &cobra.Command{
		Use:     "env RESOURCE/NAME KEY_1=VAL_1 ... KEY_N=VAL_N",
		Short:   "Update environment variables on a pod template",
		Long:    envLong,
		Example: fmt.Sprintf(envExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunEnv(f, in, out, errout, cmd, args, env, filenames)
			if err == cmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringP("containers", "c", "*", "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().StringP("from", "", "", "The name of a resource from which to inject enviroment variables")
	cmd.Flags().StringP("prefix", "", "", "Prefix to append to variable names")
	cmd.Flags().StringArrayVarP(&env, "env", "e", env, "Specify a key-value pair for an environment variable to set into each container.")
	cmd.Flags().Bool("list", false, "Display the environment and any changes in the standard format")
	cmd.Flags().Bool("resolve", false, "Show secret or configmap references when listing variables")
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().Bool("all", false, "Select all resources in the namespace of the specified resource types")
	cmd.Flags().StringSliceVarP(&filenames, "filename", "f", filenames, "Filename, directory, or URL to file to use to edit the resource.")
	cmd.Flags().Bool("overwrite", true, "If true, allow environment to be overwritten, otherwise reject updates that overwrite existing environment.")
	cmd.Flags().String("resource-version", "", "If non-empty, the labels update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.")

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

type resourceStore struct {
	secretStore    map[string]*kapi.Secret
	configMapStore map[string]*kapi.ConfigMap
}

func newResourceStore() *resourceStore {
	return &resourceStore{
		secretStore:    make(map[string]*kapi.Secret),
		configMapStore: make(map[string]*kapi.ConfigMap),
	}
}

func getSecretRefValue(f *clientcmd.Factory, store *resourceStore, secretSelector *kapi.SecretKeySelector) (string, error) {
	secret, ok := store.secretStore[secretSelector.Name]
	if !ok {
		kubeClient, err := f.Client()
		if err != nil {
			return "", err
		}
		namespace, _, err := f.DefaultNamespace()
		if err != nil {
			return "", err
		}
		secret, err = kubeClient.Secrets(namespace).Get(secretSelector.Name)
		if err != nil {
			return "", err
		}
		store.secretStore[secretSelector.Name] = secret
	}
	if data, ok := secret.Data[secretSelector.Key]; ok {
		return string(data), nil
	}
	return "", fmt.Errorf("key %s not found in secret %s", secretSelector.Key, secretSelector.Name)
}

func getConfigMapRefValue(f *clientcmd.Factory, store *resourceStore, configMapSelector *kapi.ConfigMapKeySelector) (string, error) {
	configMap, ok := store.configMapStore[configMapSelector.Name]
	if !ok {
		kubeClient, err := f.Client()
		if err != nil {
			return "", err
		}
		namespace, _, err := f.DefaultNamespace()
		if err != nil {
			return "", err
		}
		configMap, err = kubeClient.ConfigMaps(namespace).Get(configMapSelector.Name)
		if err != nil {
			return "", err
		}
		store.configMapStore[configMapSelector.Name] = configMap
	}
	if data, ok := configMap.Data[configMapSelector.Key]; ok {
		return string(data), nil
	}
	return "", fmt.Errorf("key %s not found in config map %s", configMapSelector.Key, configMapSelector.Name)
}

func getEnvVarRefValue(f *clientcmd.Factory, store *resourceStore, from *kapi.EnvVarSource, obj runtime.Object, c *kapi.Container) (string, error) {
	if from.SecretKeyRef != nil {
		return getSecretRefValue(f, store, from.SecretKeyRef)
	}

	if from.ConfigMapKeyRef != nil {
		return getConfigMapRefValue(f, store, from.ConfigMapKeyRef)
	}

	if from.FieldRef != nil {
		return fieldpath.ExtractFieldPathAsString(obj, from.FieldRef.FieldPath)
	}

	if from.ResourceFieldRef != nil {
		return fieldpath.ExtractContainerResourceValue(from.ResourceFieldRef, c)
	}

	return "", fmt.Errorf("invalid valueFrom")
}

func getEnvVarRefString(from *kapi.EnvVarSource) string {
	if from.ConfigMapKeyRef != nil {
		return fmt.Sprintf("configmap %s, key %s", from.ConfigMapKeyRef.Name, from.ConfigMapKeyRef.Key)
	}

	if from.SecretKeyRef != nil {
		return fmt.Sprintf("secret %s, key %s", from.SecretKeyRef.Name, from.SecretKeyRef.Key)
	}

	if from.FieldRef != nil {
		return fmt.Sprintf("field path %s", from.FieldRef.FieldPath)
	}

	if from.ResourceFieldRef != nil {
		containerPrefix := ""
		if from.ResourceFieldRef.ContainerName != "" {
			containerPrefix = fmt.Sprintf("%s/", from.ResourceFieldRef.ContainerName)
		}
		return fmt.Sprintf("resource field %s%s", containerPrefix, from.ResourceFieldRef.Resource)
	}

	return "invalid valueFrom"
}

// RunEnv contains all the necessary functionality for the OpenShift cli env command
// TODO: refactor to share the common "patch resource" pattern of probe
func RunEnv(f *clientcmd.Factory, in io.Reader, out, errout io.Writer, cmd *cobra.Command, args []string, envParams, filenames []string) error {
	resources, envArgs, ok := cmdutil.SplitEnvironmentFromResources(args)
	if !ok {
		return kcmdutil.UsageError(cmd, "all resources must be specified before environment changes: %s", strings.Join(args, " "))
	}
	if len(filenames) == 0 && len(resources) < 1 {
		return kcmdutil.UsageError(cmd, "one or more resources must be specified as <resource> <name> or <resource>/<name>")
	}

	containerMatch := kcmdutil.GetFlagString(cmd, "containers")
	list := kcmdutil.GetFlagBool(cmd, "list")
	resolve := kcmdutil.GetFlagBool(cmd, "resolve")
	selector := kcmdutil.GetFlagString(cmd, "selector")
	all := kcmdutil.GetFlagBool(cmd, "all")
	overwrite := kcmdutil.GetFlagBool(cmd, "overwrite")
	resourceVersion := kcmdutil.GetFlagString(cmd, "resource-version")
	outputFormat := kcmdutil.GetFlagString(cmd, "output")
	from := kcmdutil.GetFlagString(cmd, "from")
	prefix := kcmdutil.GetFlagString(cmd, "prefix")

	if list && len(outputFormat) > 0 {
		return kcmdutil.UsageError(cmd, "--list and --output may not be specified together")
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	cmdutil.WarnAboutCommaSeparation(errout, envParams, "--env")

	env, remove, err := cmdutil.ParseEnv(append(envParams, envArgs...), in)
	if err != nil {
		return err
	}

	if len(from) != 0 {
		mapper, typer := f.Object(false)
		b := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
			ContinueOnError().
			NamespaceParam(cmdNamespace).DefaultNamespace().
			FilenameParam(explicit, false, filenames...).
			SelectorParam(selector).
			ResourceTypeOrNameArgs(all, from).
			Flatten()

		one := false
		infos, err := b.Do().IntoSingular(&one).Infos()
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

	if len(prefix) != 0 {
		for i := range env {
			env[i].Name = fmt.Sprintf("%s%s", prefix, env[i].Name)
		}
	}

	mapper, typer := f.Object(false)
	b := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, false, filenames...).
		SelectorParam(selector).
		ResourceTypeOrNameArgs(all, resources...).
		Flatten()

	one := false
	infos, err := b.Do().IntoSingular(&one).Infos()
	if err != nil {
		return err
	}

	// only apply resource version locking on a single resource
	if !one && len(resourceVersion) > 0 {
		return kcmdutil.UsageError(cmd, "--resource-version may only be used with a single resource")
	}
	// Keep a copy of the original objects prior to updating their environment.
	// Used in constructing the patch(es) that will be applied in the server.
	gv := *clientConfig.GroupVersion
	oldObjects, err := resource.AsVersionedObjects(infos, gv, kapi.Codecs.LegacyCodec(gv))
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
		ok, err := f.UpdatePodSpecForObject(info.Object, func(spec *kapi.PodSpec) error {
			resolutionErrorsEncountered := false
			containers, _ := selectContainers(spec.Containers, containerMatch)
			if len(containers) == 0 {
				fmt.Fprintf(errout, "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource, info.Name, containerMatch)
				return nil
			}
			for _, c := range containers {
				if !overwrite {
					if err := validateNoOverwrites(c.Env, env); err != nil {
						errored = append(errored, info)
						return err
					}
				}

				c.Env = updateEnv(c.Env, env, remove)

				if list {
					resolveErrors := map[string][]string{}
					store := newResourceStore()

					fmt.Fprintf(out, "# %s %s, container %s\n", info.Mapping.Resource, info.Name, c.Name)
					for _, env := range c.Env {
						// Print the simple value
						if env.ValueFrom == nil {
							fmt.Fprintf(out, "%s=%s\n", env.Name, env.Value)
							continue
						}

						// Print the reference version
						if !resolve {
							fmt.Fprintf(out, "# %s from %s\n", env.Name, getEnvVarRefString(env.ValueFrom))
							continue
						}

						value, err := getEnvVarRefValue(f, store, env.ValueFrom, info.Object, c)
						// Print the resolved value
						if err == nil {
							fmt.Fprintf(out, "%s=%s\n", env.Name, value)
							continue
						}

						// Print the reference version and save the resolve error
						fmt.Fprintf(out, "# %s from %s\n", env.Name, getEnvVarRefString(env.ValueFrom))
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
						fmt.Fprintln(errout, err)
					}
				}
			}
			if resolutionErrorsEncountered {
				errored = append(errored, info)
				return errors.New("failed to retrieve valueFrom references")
			}
			return nil
		})
		if !ok {
			// This is a fallback function for objects that don't have pod spec.
			ok, err = f.UpdateObjectEnvironment(info.Object, func(vars *[]kapi.EnvVar) error {
				if vars == nil {
					return fmt.Errorf("no environment variables provided")
				}
				if !overwrite {
					if err := validateNoOverwrites(*vars, env); err != nil {
						errored = append(errored, info)
						return err
					}
				}
				*vars = updateEnv(*vars, env, remove)
				if list {
					fmt.Fprintf(out, "# %s %s\n", info.Mapping.Resource, info.Name)
					for _, env := range *vars {
						fmt.Fprintf(out, "%s=%s\n", env.Name, env.Value)
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
			fmt.Fprintf(errout, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, err)
			continue
		}
	}
	if one && skipped == len(infos) {
		return fmt.Errorf("%s/%s is not a pod or does not have a pod template", infos[0].Mapping.Resource, infos[0].Name)
	}
	if len(errored) == len(infos) {
		return cmdutil.ErrExit
	}

	if list {
		return nil
	}

	if len(outputFormat) > 0 {
		return f.PrintResourceInfos(cmd, infos, out)
	}

	objects, err := resource.AsVersionedObjects(infos, gv, kapi.Codecs.LegacyCodec(gv))
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
		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, kapi.StrategicMergePatchType, patchBytes)
		if err != nil {
			handlePodUpdateError(errout, err, "environment variables")
			failed = true
			continue
		}
		info.Refresh(obj, true)

		// make sure arguments to set or replace environment variables are set
		// before returning a successful message
		if len(env) == 0 && len(envArgs) == 0 {
			return fmt.Errorf("at least one environment variable must be provided")
		}

		shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
		kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if failed {
		return cmdutil.ErrExit
	}
	return nil
}
