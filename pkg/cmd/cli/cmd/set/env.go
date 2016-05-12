package set

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/util/strategicpatch"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	envLong = `
Update environment variables on a pod template or a build config

List environment variable definitions in one or more pods, pod templates or build
configuration.
Add, update, or remove container environment variable definitions in one or
more pod templates (within replication controllers or deployment configurations) or
build configurations.
View or modify the environment variable definitions on all containers in the
specified pods or pod templates, or just those that match a wildcard.

If "--env -" is passed, environment variables can be read from STDIN using the standard env
syntax.`

	envExample = `  # Update deployment 'registry' with a new environment variable
  %[1]s env dc/registry STORAGE_DIR=/local

  # List the environment variables defined on a build config 'sample-build'
  %[1]s env bc/sample-build --list

  # List the environment variables defined on all pods
  %[1]s env pods --all --list

  # Output modified build config in YAML, and does not alter the object on the server
  %[1]s env bc/sample-build STORAGE_DIR=/data -o yaml

  # Update all containers in all replication controllers in the project to have ENV=prod
  %[1]s env rc --all ENV=prod

  # Remove the environment variable ENV from container 'c1' in all deployment configs
  %[1]s env dc --all --containers="c1" ENV-

  # Remove the environment variable ENV from a deployment config definition on disk and
  # update the deployment config on the server
  %[1]s env -f dc.json ENV-

  # Set some of the local shell environment into a deployment config on the server
  env | grep RAILS_ | %[1]s env -e - dc/registry`
)

// NewCmdEnv implements the OpenShift cli env command
func NewCmdEnv(fullName string, f *clientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	var filenames []string
	var env []string
	cmd := &cobra.Command{
		Use:     "env RESOURCE/NAME KEY_1=VAL_1 ... KEY_N=VAL_N",
		Short:   "Update environment variables on a pod template",
		Long:    envLong,
		Example: fmt.Sprintf(envExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunEnv(f, in, out, cmd, args, env, filenames)
			if err == cmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringP("containers", "c", "*", "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().StringSliceVarP(&env, "env", "e", env, "Specify key value pairs of environment variables to set into each container.")
	cmd.Flags().Bool("list", false, "Display the environment and any changes in the standard format")
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().Bool("all", false, "Select all resources in the namespace of the specified resource types")
	cmd.Flags().StringSliceVarP(&filenames, "filename", "f", filenames, "Filename, directory, or URL to file to use to edit the resource.")
	cmd.Flags().Bool("overwrite", true, "If true, allow environment to be overwritten, otherwise reject updates that overwrite existing environment.")
	cmd.Flags().String("resource-version", "", "If non-empty, the labels update will only succeed if this is the current resource-version for the object. Only valid when specifying a single resource.")
	cmd.Flags().StringP("output", "o", "", "Display the changed objects instead of updating them. One of: json|yaml.")
	cmd.Flags().String("output-version", "", "Output the changed objects with the given version (default api-version).")

	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")

	return cmd
}

func validateNoOverwrites(meta *kapi.ObjectMeta, labels map[string]string) error {
	for key := range labels {
		if value, found := meta.Labels[key]; found {
			return fmt.Errorf("'%s' already has a value (%s), and --overwrite is false", key, value)
		}
	}
	return nil
}

// RunEnv contains all the necessary functionality for the OpenShift cli env command
// TODO: refactor to share the common "patch resource" pattern of probe
func RunEnv(f *clientcmd.Factory, in io.Reader, out io.Writer, cmd *cobra.Command, args []string, envParams, filenames []string) error {
	resources, envArgs, ok := cmdutil.SplitEnvironmentFromResources(args)
	if !ok {
		return kcmdutil.UsageError(cmd, "all resources must be specified before environment changes: %s", strings.Join(args, " "))
	}
	if len(filenames) == 0 && len(resources) < 1 {
		return kcmdutil.UsageError(cmd, "one or more resources must be specified as <resource> <name> or <resource>/<name>")
	}

	containerMatch := kcmdutil.GetFlagString(cmd, "containers")
	list := kcmdutil.GetFlagBool(cmd, "list")
	selector := kcmdutil.GetFlagString(cmd, "selector")
	all := kcmdutil.GetFlagBool(cmd, "all")
	//overwrite := kcmdutil.GetFlagBool(cmd, "overwrite")
	resourceVersion := kcmdutil.GetFlagString(cmd, "resource-version")
	outputFormat := kcmdutil.GetFlagString(cmd, "output")

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

	env, remove, err := cmdutil.ParseEnv(append(envParams, envArgs...), in)
	if err != nil {
		return err
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
	oldObjects, err := resource.AsVersionedObjects(infos, clientConfig.GroupVersion.String(), kapi.Codecs.LegacyCodec(*clientConfig.GroupVersion))
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
	for _, info := range infos {
		ok, err := f.UpdatePodSpecForObject(info.Object, func(spec *kapi.PodSpec) error {
			containers, _ := selectContainers(spec.Containers, containerMatch)
			if len(containers) == 0 {
				fmt.Fprintf(cmd.Out(), "warning: %s/%s does not have any containers matching %q\n", info.Mapping.Resource, info.Name, containerMatch)
				return nil
			}
			for _, c := range containers {
				c.Env = updateEnv(c.Env, env, remove)

				if list {
					fmt.Fprintf(out, "# %s %s, container %s\n", info.Mapping.Resource, info.Name, c.Name)
					for _, env := range c.Env {
						// if env.ValueFrom != nil && env.ValueFrom.FieldRef != nil {
						// 	fmt.Fprintf(cmd.Out(), "%s= # calculated from pod %s %s\n", env.Name, env.ValueFrom.FieldRef.FieldPath, env.ValueFrom.FieldRef.APIVersion)
						// 	continue
						// }
						fmt.Fprintf(out, "%s=%s\n", env.Name, env.Value)

					}
				}
			}
			return nil
		})
		if !ok {
			// This is a fallback function for objects that don't have pod spec.
			ok, err = f.UpdateObjectEnvironment(info.Object, func(vars *[]kapi.EnvVar) error {
				if vars == nil {
					return fmt.Errorf("no environment variables provided")
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
			fmt.Fprintf(cmd.Out(), "error: %s/%s %v\n", info.Mapping.Resource, info.Name, err)
			continue
		}
	}
	if one && skipped == len(infos) {
		return fmt.Errorf("%s/%s is not a pod or does not have a pod template", infos[0].Mapping.Resource, infos[0].Name)
	}

	if list {
		return nil
	}

	if len(outputFormat) != 0 {
		outputVersion, err := kcmdutil.OutputVersion(cmd, clientConfig.GroupVersion)
		if err != nil {
			return err
		}
		objects, err := resource.AsVersionedObjects(infos, outputVersion.String(), kapi.Codecs.LegacyCodec(outputVersion))
		if err != nil {
			return err
		}
		if len(objects) != len(infos) {
			return fmt.Errorf("could not convert all objects to API version %q", outputVersion)
		}
		p, _, err := kubectl.GetPrinter(outputFormat, "")
		if err != nil {
			return err
		}
		for _, object := range objects {
			if err := p.PrintObj(object, out); err != nil {
				return err
			}
		}
		return nil
	}

	objects, err := resource.AsVersionedObjects(infos, clientConfig.GroupVersion.String(), kapi.Codecs.LegacyCodec(*clientConfig.GroupVersion))
	if err != nil {
		return err
	}
	if len(objects) != len(infos) {
		return fmt.Errorf("could not convert all objects to API version %q", clientConfig.GroupVersion)
	}

	failed := false
	for i, info := range infos {
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
			handlePodUpdateError(cmd.Out(), err, "environment variables")
			failed = true
			continue
		}
		info.Refresh(obj, true)

		shortOutput := kcmdutil.GetFlagString(cmd, "output") == "name"
		kcmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, "updated")
	}
	if failed {
		return cmdutil.ErrExit
	}
	return nil
}
