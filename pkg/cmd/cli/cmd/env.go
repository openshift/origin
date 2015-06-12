package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/spf13/cobra"

	//ocutil "github.com/openshift/origin/pkg/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	envLong = `Update the environment on a pod

Each container in a pod may have its own environment variable definitions. This command can
add, update, or remove environment variables from containers in pods or any object that has
a pod template (replication controllers or deployment configurations). You can specify a
single object or multiple, and alter environment on all containers or just those that match
a wildcard.

If "--env -" is passed, environment variables can be read from STDIN using the standard env
syntax.`

	envExample = `  // Update deployment 'registry' with a new environment variable
  $ %[1]s env dc/registry STORAGE_DIR=/local

  // List the environment variables defined on a deployment 'registry'
  $ %[1]s env dc/registry --list

  // List the environment variables defined on all pods
  $ %[1]s env pods --all --list

  // Output a YAML object with updated enviroment for deployment 'registry'
  // Does not alter the object on the server
  $ %[1]s env dc/registry STORAGE_DIR=/local -o yaml

  // Update all replication controllers in the project to have ENV=prod
  $ %[1]s env replicationControllers --all ENV=prod

  // Remove the enviroment variable ENV from all deployment configs
  $ %[1]s env deploymentConfigs --all ENV-

  // Remove the enviroment variable ENV from a pod definition on disk and update the pod on the server
  $ %[1]s env -f pod.json ENV-

  // Set some of the local shell environment into a deployment on the server
  $ env | grep RAILS_ | %[1]s env -e - dc/registry`
)

// NewCmdEnv implements the OpenShift cli env command
func NewCmdEnv(fullName string, f *clientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	var filenames util.StringList
	var env util.StringList
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
	exists := util.NewStringSet()
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
func RunEnv(f *clientcmd.Factory, in io.Reader, out io.Writer, cmd *cobra.Command, args []string, envParams, filenames util.StringList) error {
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

	cmdNamespace, err := f.DefaultNamespace()
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
		FilenameParam(filenames...).
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
			containers := selectContainers(spec.Containers, containerMatch)
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
		data, err := info.Mapping.Codec.Encode(info.Object)
		if err != nil {
			fmt.Fprintf(cmd.Out(), "Error: %v\n", err)
			failed = true
			continue
		}
		obj, err := resource.NewHelper(info.Client, info.Mapping).Update(info.Namespace, info.Name, true, data)
		if err != nil {
			handleUpdateError(cmd.Out(), err)
			failed = true
			continue
		}
		info.Refresh(obj, true)
		fmt.Fprintf(out, "%s/%s\n", info.Mapping.Resource, info.Name)
	}
	if failed {
		return errExit
	}
	return nil
}

func updateEnv(existing []kapi.EnvVar, env []kapi.EnvVar, remove []string) []kapi.EnvVar {
	out := []kapi.EnvVar{}
	covered := util.NewStringSet(remove...)
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

func selectContainers(containers []kapi.Container, spec string) []*kapi.Container {
	out := []*kapi.Container{}
	for i, c := range containers {
		if selectString(c.Name, spec) {
			out = append(out, &containers[i])
		}
	}
	return out
}

// selectString returns true if the provided string matches spec, where spec is a string with
// a non-greedy '*' wildcard operator.
// TODO: turn into a regex and handle greedy matches and backtracking.
func selectString(s, spec string) bool {
	if spec == "*" {
		return true
	}
	if !strings.Contains(spec, "*") {
		return s == spec
	}

	pos := 0
	match := true
	parts := strings.Split(spec, "*")
	for i, part := range parts {
		if len(part) == 0 {
			continue
		}
		next := strings.Index(s[pos:], part)
		switch {
		// next part not in string
		case next < pos:
			fallthrough
		// first part does not match start of string
		case i == 0 && pos != 0:
			fallthrough
		// last part does not exactly match remaining part of string
		case i == (len(parts)-1) && len(s) != (len(part)+next):
			match = false
			break
		default:
			pos = next
		}
	}
	return match
}

func handleUpdateError(out io.Writer, err error) {
	errored := false
	if statusError, ok := err.(*errors.StatusError); ok && errors.IsInvalid(err) {
		errorDetails := statusError.Status().Details
		if errorDetails.Kind == "Pod" {
			for _, cause := range errorDetails.Causes {
				if cause.Field == "spec" && strings.Contains(cause.Message, "may not update fields other than") {
					fmt.Fprintf(out, "Error updating pod %q: may not update environment variables in a pod directly\n", errorDetails.ID)
					errored = true
					break
				}
			}
		}
	}

	if !errored {
		fmt.Fprintf(out, "Error: %v\n", err)
	}
}
