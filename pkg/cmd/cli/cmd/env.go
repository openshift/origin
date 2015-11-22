package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	envLong = `
Update environment variables on a pod template

List environment variable definitions in one or more pods or pod templates.
Add, update, or remove container environment variable definitions in one or
more pod templates (within replication controllers or deployment configurations).
View or modify the environment variable definitions on all containers in the
specified pods or pod templates, or just those that match a wildcard.

If "--env -" is passed, environment variables can be read from STDIN using the standard env
syntax.`

	envExample = `  # Update deployment 'registry' with a new environment variable
  $ %[1]s env dc/registry STORAGE_DIR=/local

  # List the environment variables defined on a deployment config 'registry'
  $ %[1]s env dc/registry --list

  # List the environment variables defined on all pods
  $ %[1]s env pods --all --list

  # Output modified deployment config in YAML, and does not alter the object on the server
  $ %[1]s env dc/registry STORAGE_DIR=/data -o yaml

  # Update all containers in all replication controllers in the project to have ENV=prod
  $ %[1]s env rc --all ENV=prod

  # Remove the environment variable ENV from container 'c1' in all deployment configs
  $ %[1]s env dc --all --containers="c1" ENV-

  # Remove the environment variable ENV from a deployment config definition on disk and
  # update the deployment config on the server
  $ %[1]s env -f dc.json ENV-

  # Set some of the local shell environment into a deployment config on the server
  $ env | grep RAILS_ | %[1]s env -e - dc/registry`
)

// NewCmdEnv implements the OpenShift cli env command
func NewCmdEnv(fullName string, f *clientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	var filenames kutil.StringList
	var env kutil.StringList
	cmd := &cobra.Command{
		Use:     "env RESOURCE/NAME KEY_1=VAL_1 ... KEY_N=VAL_N",
		Short:   "Update the environment on a resource with a pod template",
		Long:    envLong,
		Example: fmt.Sprintf(envExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunEnv(f, in, out, cmd, args, env, filenames)
			if err == errExit {
				os.Exit(1)
			}
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().StringP("containers", "c", "*", "The names of containers in the selected pod templates to change - may use wildcards")
	cmd.Flags().VarP(&env, "env", "e", "Specify key value pairs of environment variables to set into each container.")
	cmd.Flags().Bool("list", false, "Display the environment and any changes in the standard format")
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().Bool("all", false, "select all resources in the namespace of the specified resource types")
	cmd.Flags().VarP(&filenames, "filename", "f", "Filename, directory, or URL to file to use to edit the resource.")
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

func parseEnv(spec []string, defaultReader io.Reader) ([]kapi.EnvVar, []string, error) {
	env := []kapi.EnvVar{}
	exists := sets.NewString()
	var remove []string
	for _, envSpec := range spec {
		switch {
		case envSpec == "-":
			if defaultReader == nil {
				return nil, nil, fmt.Errorf("when '-' is used, STDIN must be open")
			}
			fileEnv, err := readEnv(defaultReader)
			if err != nil {
				return nil, nil, err
			}
			env = append(env, fileEnv...)
		case strings.Index(envSpec, "=") != -1:
			parts := strings.SplitN(envSpec, "=", 2)
			if len(parts) != 2 {
				return nil, nil, fmt.Errorf("invalid environment variable: %v", envSpec)
			}
			exists.Insert(parts[0])
			env = append(env, kapi.EnvVar{
				Name:  parts[0],
				Value: parts[1],
			})
		case strings.HasSuffix(envSpec, "-"):
			remove = append(remove, envSpec[:len(envSpec)-1])
		default:
			return nil, nil, fmt.Errorf("unknown environment variable: %v", envSpec)
		}
	}
	for _, removeLabel := range remove {
		if _, found := exists[removeLabel]; found {
			return nil, nil, fmt.Errorf("can not both modify and remove an environment variable in the same command")
		}
	}
	return env, remove, nil
}

func readEnv(r io.Reader) ([]kapi.EnvVar, error) {
	env := []kapi.EnvVar{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		envSpec := scanner.Text()
		if pos := strings.Index(envSpec, "#"); pos != -1 {
			envSpec = envSpec[:pos]
		}
		if strings.Index(envSpec, "=") != -1 {
			parts := strings.SplitN(envSpec, "=", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid environment variable: %v", envSpec)
			}
			env = append(env, kapi.EnvVar{
				Name:  parts[0],
				Value: parts[1],
			})
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return env, nil
}

// RunEnv contains all the necessary functionality for the OpenShift cli env command
func RunEnv(f *clientcmd.Factory, in io.Reader, out io.Writer, cmd *cobra.Command, args []string, envParams, filenames kutil.StringList) error {
	resources, envArgs := []string{}, []string{}
	first := true
	for _, s := range args {
		isEnv := strings.Contains(s, "=") || strings.HasSuffix(s, "-")
		switch {
		case first && isEnv:
			first = false
			fallthrough
		case !first && isEnv:
			envArgs = append(envArgs, s)
		case first && !isEnv:
			resources = append(resources, s)
		case !first && !isEnv:
			return cmdutil.UsageError(cmd, "all resources must be specified before environment changes: %s", s)
		}
	}
	if len(filenames) == 0 && len(resources) < 1 {
		return cmdutil.UsageError(cmd, "one or more resources must be specified as <resource> <name> or <resource>/<name>")
	}

	containerMatch := cmdutil.GetFlagString(cmd, "containers")
	list := cmdutil.GetFlagBool(cmd, "list")
	selector := cmdutil.GetFlagString(cmd, "selector")
	all := cmdutil.GetFlagBool(cmd, "all")
	//overwrite := cmdutil.GetFlagBool(cmd, "overwrite")
	resourceVersion := cmdutil.GetFlagString(cmd, "resource-version")
	outputFormat := cmdutil.GetFlagString(cmd, "output")

	if list && len(outputFormat) > 0 {
		return cmdutil.UsageError(cmd, "--list and --output may not be specified together")
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	outputVersion := cmdutil.OutputVersion(cmd, clientConfig.Version)

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	env, remove, err := parseEnv(append(envParams, envArgs...), in)
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	b := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, filenames...).
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
		return cmdutil.UsageError(cmd, "--resource-version may only be used with a single resource")
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
			skipped++
			continue
		}
		if err != nil {
			fmt.Fprintf(cmd.Out(), "error: %s/%s %v\n", info.Mapping.Resource, info.Name, err)
			continue
		}
	}
	if one && skipped == len(infos) {
		return fmt.Errorf("the %s %s is not a pod or does not have a pod template", infos[0].Mapping.Resource, infos[0].Name)
	}

	if list {
		return nil
	}

	objects, err := resource.AsVersionedObject(infos, false, outputVersion)
	if err != nil {
		return err
	}

	if len(outputFormat) != 0 {
		p, _, err := kubectl.GetPrinter(outputFormat, "")
		if err != nil {
			return err
		}
		return p.PrintObj(objects, out)
	}

	failed := false
	for _, info := range infos {
		obj, err := resource.NewHelper(info.Client, info.Mapping).Replace(info.Namespace, info.Name, true, info.Object)
		if err != nil {
			handlePodUpdateError(cmd.Out(), err, "environment variables")
			failed = true
			continue
		}
		info.Refresh(obj, true)

		shortOutput := cmdutil.GetFlagString(cmd, "output") == "name"
		cmdutil.PrintSuccess(mapper, shortOutput, out, info.Mapping.Resource, info.Name, "updated")
	}
	if failed {
		return errExit
	}
	return nil
}

func updateEnv(existing []kapi.EnvVar, env []kapi.EnvVar, remove []string) []kapi.EnvVar {
	out := []kapi.EnvVar{}
	covered := sets.NewString(remove...)
	for _, e := range existing {
		if covered.Has(e.Name) {
			continue
		}
		newer, ok := findEnv(env, e.Name)
		if ok {
			covered.Insert(e.Name)
			out = append(out, newer)
			continue
		}
		out = append(out, e)
	}
	for _, e := range env {
		if covered.Has(e.Name) {
			continue
		}
		covered.Insert(e.Name)
		out = append(out, e)
	}
	return out
}

func findEnv(env []kapi.EnvVar, name string) (kapi.EnvVar, bool) {
	for _, e := range env {
		if e.Name == name {
			return e, true
		}
	}
	return kapi.EnvVar{}, false
}
