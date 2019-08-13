package kubectlwrappers

import (
	"bufio"
	"flag"
	"fmt"
	"path"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/annotate"
	"k8s.io/kubernetes/pkg/kubectl/cmd/apiresources"
	"k8s.io/kubernetes/pkg/kubectl/cmd/apply"
	"k8s.io/kubernetes/pkg/kubectl/cmd/attach"
	kcmdauth "k8s.io/kubernetes/pkg/kubectl/cmd/auth"
	"k8s.io/kubernetes/pkg/kubectl/cmd/autoscale"
	"k8s.io/kubernetes/pkg/kubectl/cmd/clusterinfo"
	"k8s.io/kubernetes/pkg/kubectl/cmd/completion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/config"
	"k8s.io/kubernetes/pkg/kubectl/cmd/convert"
	"k8s.io/kubernetes/pkg/kubectl/cmd/cp"
	kcreate "k8s.io/kubernetes/pkg/kubectl/cmd/create"
	"k8s.io/kubernetes/pkg/kubectl/cmd/delete"
	"k8s.io/kubernetes/pkg/kubectl/cmd/describe"
	"k8s.io/kubernetes/pkg/kubectl/cmd/edit"
	"k8s.io/kubernetes/pkg/kubectl/cmd/exec"
	"k8s.io/kubernetes/pkg/kubectl/cmd/explain"
	kget "k8s.io/kubernetes/pkg/kubectl/cmd/get"
	"k8s.io/kubernetes/pkg/kubectl/cmd/label"
	"k8s.io/kubernetes/pkg/kubectl/cmd/patch"
	"k8s.io/kubernetes/pkg/kubectl/cmd/plugin"
	"k8s.io/kubernetes/pkg/kubectl/cmd/portforward"
	"k8s.io/kubernetes/pkg/kubectl/cmd/proxy"
	"k8s.io/kubernetes/pkg/kubectl/cmd/replace"
	"k8s.io/kubernetes/pkg/kubectl/cmd/run"
	"k8s.io/kubernetes/pkg/kubectl/cmd/scale"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kwait "k8s.io/kubernetes/pkg/kubectl/cmd/wait"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/oc/pkg/cli/create"
	cmdutil "github.com/openshift/oc/pkg/helpers/cmd"
)

func adjustCmdExamples(cmd *cobra.Command, fullName string, name string) {
	for _, subCmd := range cmd.Commands() {
		adjustCmdExamples(subCmd, fullName, cmd.Name())
	}
	cmd.Example = strings.Replace(cmd.Example, "kubectl", fullName, -1)
	tabbing := "  "
	examples := []string{}
	scanner := bufio.NewScanner(strings.NewReader(cmd.Example))
	for scanner.Scan() {
		examples = append(examples, tabbing+strings.TrimSpace(scanner.Text()))
	}
	cmd.Example = strings.Join(examples, "\n")
}

// NewCmdGet is a wrapper for the Kubernetes cli get command
func NewCmdGet(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(kget.NewCmdGet(fullName, f, streams)))
}

// NewCmdReplace is a wrapper for the Kubernetes cli replace command
func NewCmdReplace(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(replace.NewCmdReplace(f, streams)))
}

func NewCmdClusterInfo(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(clusterinfo.NewCmdClusterInfo(f, streams)))
}

// NewCmdPatch is a wrapper for the Kubernetes cli patch command
func NewCmdPatch(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(patch.NewCmdPatch(f, streams)))
}

// NewCmdDelete is a wrapper for the Kubernetes cli delete command
func NewCmdDelete(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(delete.NewCmdDelete(f, streams)))
}

// NewCmdCreate is a wrapper for the Kubernetes cli create command
func NewCmdCreate(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(kcreate.NewCmdCreate(f, streams)))

	// create subcommands
	cmd.AddCommand(create.NewCmdCreateRoute(fullName, f, streams))
	cmd.AddCommand(create.NewCmdCreateDeploymentConfig(create.DeploymentConfigRecommendedName, fullName+" create "+create.DeploymentConfigRecommendedName, f, streams))
	cmd.AddCommand(create.NewCmdCreateClusterQuota(create.ClusterQuotaRecommendedName, fullName+" create "+create.ClusterQuotaRecommendedName, f, streams))

	cmd.AddCommand(create.NewCmdCreateUser(create.UserRecommendedName, fullName+" create "+create.UserRecommendedName, f, streams))
	cmd.AddCommand(create.NewCmdCreateIdentity(create.IdentityRecommendedName, fullName+" create "+create.IdentityRecommendedName, f, streams))
	cmd.AddCommand(create.NewCmdCreateUserIdentityMapping(create.UserIdentityMappingRecommendedName, fullName+" create "+create.UserIdentityMappingRecommendedName, f, streams))
	cmd.AddCommand(create.NewCmdCreateImageStream(create.ImageStreamRecommendedName, fullName+" create "+create.ImageStreamRecommendedName, f, streams))
	cmd.AddCommand(create.NewCmdCreateImageStreamTag(create.ImageStreamTagRecommendedName, fullName+" create "+create.ImageStreamTagRecommendedName, f, streams))

	adjustCmdExamples(cmd, fullName, "create")

	return cmd
}

var (
	completionLong = templates.LongDesc(`
		This command prints shell code which must be evaluated to provide interactive
		completion of %s commands.`)

	completionExample = templates.Examples(`
		# Generate the %s completion code for bash
	  %s completion bash > bash_completion.sh
	  source bash_completion.sh

	  # The above example depends on the bash-completion framework.
	  # It must be sourced before sourcing the openshift cli completion,
		# i.e. on the Mac:

	  brew install bash-completion
	  source $(brew --prefix)/etc/bash_completion
	  %s completion bash > bash_completion.sh
	  source bash_completion.sh

	  # In zsh*, the following will load openshift cli zsh completion:
	  source <(%s completion zsh)

	  * zsh completions are only supported in versions of zsh >= 5.2`)
)

func NewCmdCompletion(fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	cmdHelpName := fullName

	if strings.HasSuffix(fullName, "completion") {
		cmdHelpName = "openshift"
	}

	cmd := completion.NewCmdCompletion(streams.Out, "\n")
	cmd.Long = fmt.Sprintf(completionLong, cmdHelpName)
	cmd.Example = fmt.Sprintf(completionExample, cmdHelpName, cmdHelpName, cmdHelpName, cmdHelpName)
	// mark all statically included flags as hidden to prevent them appearing in completions
	cmd.PreRun = func(c *cobra.Command, _ []string) {
		pflag.CommandLine.VisitAll(func(flag *pflag.Flag) {
			flag.Hidden = true
		})
		hideGlobalFlags(c.Root(), flag.CommandLine)
	}
	return cmd
}

// hideGlobalFlags marks any flag that is in the global flag set as
// hidden to prevent completion from varying by platform due to conditional
// includes. This means that some completions will not be possible unless
// they are registered in cobra instead of being added to flag.CommandLine.
func hideGlobalFlags(c *cobra.Command, fs *flag.FlagSet) {
	fs.VisitAll(func(flag *flag.Flag) {
		if f := c.PersistentFlags().Lookup(flag.Name); f != nil {
			f.Hidden = true
		}
		if f := c.LocalFlags().Lookup(flag.Name); f != nil {
			f.Hidden = true
		}
	})
	for _, child := range c.Commands() {
		hideGlobalFlags(child, fs)
	}
}

// NewCmdExec is a wrapper for the Kubernetes cli exec command
func NewCmdExec(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(exec.NewCmdExec(f, streams)))
	cmd.Use = "exec [flags] POD [-c CONTAINER] -- COMMAND [args...]"
	return cmd
}

// NewCmdPortForward is a wrapper for the Kubernetes cli port-forward command
func NewCmdPortForward(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(portforward.NewCmdPortForward(f, streams)))
}

var (
	describeLong = templates.LongDesc(`
		Show details of a specific resource

		This command joins many API calls together to form a detailed description of a
		given resource.`)

	describeExample = templates.Examples(`
		# Provide details about the ruby-22-centos7 image repository
	  %[1]s describe imageRepository ruby-22-centos7

	  # Provide details about the ruby-sample-build build configuration
	  %[1]s describe bc ruby-sample-build`)
)

// NewCmdDescribe is a wrapper for the Kubernetes cli describe command
func NewCmdDescribe(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := describe.NewCmdDescribe(fullName, f, streams)
	cmd.Long = describeLong
	cmd.Example = fmt.Sprintf(describeExample, fullName)
	return cmd
}

// NewCmdProxy is a wrapper for the Kubernetes cli proxy command
func NewCmdProxy(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(proxy.NewCmdProxy(f, streams)))
}

var (
	scaleLong = templates.LongDesc(`
		Set a new size for a deployment or replication controller

		Scale also allows users to specify one or more preconditions for the scale action.
		If --current-replicas or --resource-version is specified, it is validated before the
		scale is attempted, and it is guaranteed that the precondition holds true when the
		scale is sent to the server.

		Note that scaling a deployment configuration with no deployments will update the
		desired replicas in the configuration template.

		Supported resources:
		%q`)

	scaleExample = templates.Examples(`
		# Scale replication controller named 'foo' to 3.
	  %[1]s scale --replicas=3 replicationcontrollers foo

	  # If the replication controller named foo's current size is 2, scale foo to 3.
	  %[1]s scale --current-replicas=2 --replicas=3 replicationcontrollers foo

	  # Scale the latest deployment of 'bar'. In case of no deployment, bar's template
	  # will be scaled instead.
	  %[1]s scale --replicas=10 dc bar`)
)

// NewCmdScale is a wrapper for the Kubernetes cli scale command
func NewCmdScale(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := scale.NewCmdScale(f, streams)
	cmd.ValidArgs = append(cmd.ValidArgs, "deploymentconfig")
	cmd.Short = "Change the number of pods in a deployment"
	cmd.Long = fmt.Sprintf(scaleLong, cmd.ValidArgs)
	cmd.Example = fmt.Sprintf(scaleExample, fullName)
	return cmd
}

var (
	autoScaleLong = templates.LongDesc(`
		Autoscale a deployment config or replication controller.

		Looks up a deployment config or replication controller by name and creates an autoscaler that uses
		this deployment config or replication controller as a reference. An autoscaler can automatically
		increase or decrease number of pods deployed within the system as needed.`)

	autoScaleExample = templates.Examples(`
		# Auto scale a deployment config "foo", with the number of pods between 2 to
		# 10, target CPU utilization at a default value that server applies:
	  %[1]s autoscale dc/foo --min=2 --max=10

	  # Auto scale a replication controller "foo", with the number of pods between
		# 1 to 5, target CPU utilization at 80%%
	  %[1]s autoscale rc/foo --max=5 --cpu-percent=80`)
)

// NewCmdAutoscale is a wrapper for the Kubernetes cli autoscale command
func NewCmdAutoscale(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := autoscale.NewCmdAutoscale(f, streams)
	cmd.Short = "Autoscale a deployment config, deployment, replication controller, or replica set"
	cmd.Long = autoScaleLong
	cmd.Example = fmt.Sprintf(autoScaleExample, fullName)
	cmd.ValidArgs = append(cmd.ValidArgs, "deploymentconfig")
	return cmd
}

var (
	runLong = templates.LongDesc(`
		Create and run a particular image, possibly replicated

		Creates a deployment config to manage the created container(s). You can choose to run in the
		foreground for an interactive container execution.  You may pass 'run/v1' to
		--generator to create a replication controller instead of a deployment config.`)

	runExample = templates.Examples(`
		# Start a single instance of nginx.
		%[1]s run nginx --image=nginx

		# Start a single instance of hazelcast and let the container expose port 5701 .
		%[1]s run hazelcast --image=hazelcast --port=5701

		# Start a single instance of hazelcast and set environment variables "DNS_DOMAIN=cluster"
		# and "POD_NAMESPACE=default" in the container.
		%[1]s run hazelcast --image=hazelcast --env="DNS_DOMAIN=cluster" --env="POD_NAMESPACE=default"

		# Start a replicated instance of nginx.
		%[1]s run nginx --image=nginx --replicas=5

		# Dry run. Print the corresponding API objects without creating them.
		%[1]s run nginx --image=nginx --dry-run

		# Start a single instance of nginx, but overload the spec of the deployment config with
		# a partial set of values parsed from JSON.
		%[1]s run nginx --image=nginx --overrides='{ "apiVersion": "v1", "spec": { ... } }'

		# Start a pod of busybox and keep it in the foreground, don't restart it if it exits.
		%[1]s run -i -t busybox --image=busybox --restart=Never

		# Start the nginx container using the default command, but use custom arguments (arg1 .. argN)
		# for that command.
		%[1]s run nginx --image=nginx -- <arg1> <arg2> ... <argN>

		# Start the nginx container using a different command and custom arguments.
		%[1]s run nginx --image=nginx --command -- <cmd> <arg1> ... <argN>

		# Start the job to compute π to 2000 places and print it out.
		%[1]s run pi --image=perl --restart=OnFailure -- perl -Mbignum=bpi -wle 'print bpi(2000)'

		# Start the cron job to compute π to 2000 places and print it out every 5 minutes.
		%[1]s run pi --schedule="0/5 * * * ?" --image=perl --restart=OnFailure -- perl -Mbignum=bpi -wle 'print bpi(2000)'`)
)

// NewCmdRun is a wrapper for the Kubernetes cli run command
func NewCmdRun(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := run.NewCmdRun(f, streams)
	cmd.Long = runLong
	cmd.Example = fmt.Sprintf(runExample, fullName)
	cmd.Flags().Set("generator", "")
	cmd.Flag("generator").Usage = "The name of the API generator to use.  Default is 'deploymentconfig/v1' if --restart=Always, otherwise the default is 'run-pod/v1'."
	cmd.Flag("generator").DefValue = ""
	cmd.Flag("generator").Changed = false
	return cmd
}

// NewCmdAttach is a wrapper for the Kubernetes cli attach command
func NewCmdAttach(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(attach.NewCmdAttach(f, streams)))
}

// NewCmdAnnotate is a wrapper for the Kubernetes cli annotate command
func NewCmdAnnotate(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(annotate.NewCmdAnnotate(fullName, f, streams)))
}

// NewCmdLabel is a wrapper for the Kubernetes cli label command
func NewCmdLabel(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(label.NewCmdLabel(f, streams)))
}

// NewCmdApply is a wrapper for the Kubernetes cli apply command
func NewCmdApply(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(apply.NewCmdApply(fullName, f, streams)))
}

// NewCmdExplain is a wrapper for the Kubernetes cli explain command
func NewCmdExplain(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(explain.NewCmdExplain(fullName, f, streams)))
}

// NewCmdConvert is a wrapper for the Kubernetes cli convert command
func NewCmdConvert(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(convert.NewCmdConvert(f, streams)))
}

var (
	editLong = templates.LongDesc(`
		Edit a resource from the default editor

		The edit command allows you to directly edit any API resource you can retrieve via the
		command line tools. It will open the editor defined by your OC_EDITOR, or EDITOR environment
		variables, or fall back to 'vi' for Linux or 'notepad' for Windows. You can edit multiple
		objects, although changes are applied one at a time. The command accepts filenames as well
		as command line arguments, although the files you point to must be previously saved versions
		of resources.

		The files to edit will be output in the default API version, or a version specified
		by --output-version. The default format is YAML - if you would like to edit in JSON
		pass -o json. The flag --windows-line-endings can be used to force Windows line endings,
		otherwise the default for your operating system will be used.

		In the event an error occurs while updating, a temporary file will be created on disk
		that contains your unapplied changes. The most common error when updating a resource
		is another editor changing the resource on the server. When this occurs, you will have
		to apply your changes to the newer version of the resource, or update your temporary
		saved copy to include the latest resource version.`)

	editExample = templates.Examples(`
		# Edit the service named 'docker-registry':
	  %[1]s edit svc/docker-registry

	  # Edit the DeploymentConfig named 'my-deployment':
	  %[1]s edit dc/my-deployment

	  # Use an alternative editor
	  OC_EDITOR="nano" %[1]s edit dc/my-deployment

	  # Edit the service 'docker-registry' in JSON using the v1 API format:
	  %[1]s edit svc/docker-registry --output-version=v1 -o json`)
)

// NewCmdEdit is a wrapper for the Kubernetes cli edit command
func NewCmdEdit(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := edit.NewCmdEdit(f, streams)
	cmd.Long = editLong
	cmd.Example = fmt.Sprintf(editExample, fullName)
	return cmd
}

var (
	configLong = templates.LongDesc(`
		Manage the client config files

		The client stores configuration in the current user's home directory (under the .kube directory as
		config). When you login the first time, a new config file is created, and subsequent project changes with the
		'project' command will set the current context. These subcommands allow you to manage the config directly.

		Reference: https://github.com/kubernetes/kubernetes/blob/master/docs/user-guide/kubeconfig-file.md`)

	configExample = templates.Examples(`
		# Change the config context to use
	  %[1]s %[2]s use-context my-context

	  # Set the value of a config preference
	  %[1]s %[2]s set preferences.some true`)
)

// NewCmdConfig is a wrapper for the Kubernetes cli config command
func NewCmdConfig(fullName, name string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	pathOptions := &kclientcmd.PathOptions{
		GlobalFile:       kclientcmd.RecommendedHomeFile,
		EnvVar:           kclientcmd.RecommendedConfigPathEnvVar,
		ExplicitFileFlag: genericclioptions.OpenShiftKubeConfigFlagName,

		GlobalFileSubpath: path.Join(kclientcmd.RecommendedHomeDir, kclientcmd.RecommendedFileName),

		LoadingRules: kclientcmd.NewDefaultClientConfigLoadingRules(),
	}
	pathOptions.LoadingRules.DoNotResolvePaths = true

	cmd := config.NewCmdConfig(f, pathOptions, streams)
	cmd.Short = "Change configuration files for the client"
	cmd.Long = configLong
	cmd.Example = fmt.Sprintf(configExample, fullName, name)
	// normalize long descs and examples
	// TODO remove when normalization is moved upstream
	templates.NormalizeAll(cmd)
	adjustCmdExamples(cmd, fullName, name)
	return cmd
}

// NewCmdCp is a wrapper for the Kubernetes cli cp command
func NewCmdCp(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(cp.NewCmdCp(f, streams)))
}

func NewCmdWait(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return kwait.NewCmdWait(f, streams)
}

func NewCmdAuth(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(kcmdauth.NewCmdAuth(f, streams)))
}

func NewCmdPlugin(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	// list of accepted plugin executable filename prefixes that we will look for
	// when executing a plugin. Order matters here, we want to first see if a user
	// has prefixed their plugin with "oc-", before defaulting to upstream behavior.
	plugin.ValidPluginFilenamePrefixes = []string{"oc", "kubectl"}
	return plugin.NewCmdPlugin(f, streams)
}

func NewCmdApiResources(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(apiresources.NewCmdAPIResources(f, streams)))
}

func NewCmdApiVersions(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	return cmdutil.ReplaceCommandName("kubectl", fullName, templates.Normalize(apiresources.NewCmdAPIVersions(f, streams)))
}
