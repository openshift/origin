package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdauth "k8s.io/kubernetes/pkg/kubectl/cmd/auth"
	"k8s.io/kubernetes/pkg/kubectl/cmd/config"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/cmd/create"
	cmdconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func adjustCmdExamples(cmd *cobra.Command, parentName string, name string) {
	for _, subCmd := range cmd.Commands() {
		adjustCmdExamples(subCmd, parentName, cmd.Name())
	}
	cmd.Example = strings.Replace(cmd.Example, "kubectl", parentName, -1)
	tabbing := "  "
	examples := []string{}
	scanner := bufio.NewScanner(strings.NewReader(cmd.Example))
	for scanner.Scan() {
		examples = append(examples, tabbing+strings.TrimSpace(scanner.Text()))
	}
	cmd.Example = strings.Join(examples, "\n")
}

var (
	getLong = templates.LongDesc(`
		Display one or many resources

		Possible resources include builds, buildConfigs, services, pods, etc. To see
		a list of common resources, use '%[1]s get'. Some resources may omit advanced
		details that you can see with '-o wide'.  If you want an even more detailed
		view, use '%[1]s describe'.`)

	getExample = templates.Examples(`
		# List all pods in ps output format.
		%[1]s get pods

		# List a single replication controller with specified ID in ps output format.
		%[1]s get rc redis

		# List all pods and show more details about them.
		%[1]s get -o wide pods

		# List a single pod in JSON output format.
		%[1]s get -o json pod redis-pod

		# Return only the status value of the specified pod.
		%[1]s get -o template pod redis-pod --template={{.currentState.status}}`)
)

// NewCmdGet is a wrapper for the Kubernetes cli get command
func NewCmdGet(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdGet(f, out, errOut)
	cmd.Long = fmt.Sprintf(getLong, fullName)
	cmd.Example = fmt.Sprintf(getExample, fullName)
	cmd.SuggestFor = []string{"list"}
	return cmd
}

var (
	replaceLong = templates.LongDesc(`
		Replace a resource by filename or stdin

		JSON and YAML formats are accepted.`)

	replaceExample = templates.Examples(`
		# Replace a pod using the data in pod.json.
	  %[1]s replace -f pod.json

	  # Replace a pod based on the JSON passed into stdin.
	  cat pod.json | %[1]s replace -f -

	  # Force replace, delete and then re-create the resource
	  %[1]s replace --force -f pod.json`)
)

// NewCmdReplace is a wrapper for the Kubernetes cli replace command
func NewCmdReplace(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdReplace(f, out)
	cmd.Long = replaceLong
	cmd.Example = fmt.Sprintf(replaceExample, fullName)
	return cmd
}

var (
	patchLong = templates.LongDesc(`
		Update field(s) of a resource using strategic merge patch

		JSON and YAML formats are accepted.`)

	patchExample = templates.Examples(`
		# Partially update a node using strategic merge patch
  	%[1]s patch node k8s-node-1 -p '{"spec":{"unschedulable":true}}'`)
)

// NewCmdPatch is a wrapper for the Kubernetes cli patch command
func NewCmdPatch(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdPatch(f, out)
	cmd.Long = patchLong
	cmd.Example = fmt.Sprintf(patchExample, fullName)
	return cmd
}

var (
	deleteLong = templates.LongDesc(`
		Delete a resource

		JSON and YAML formats are accepted.

		If both a filename and command line arguments are passed, the command line
		arguments are used and the filename is ignored.

		Note that the delete command does NOT do resource version checks, so if someone
		submits an update to a resource right when you submit a delete, their update
		will be lost along with the rest of the resource.`)

	deleteExample = templates.Examples(`
		# Delete a pod using the type and ID specified in pod.json.
	  %[1]s delete -f pod.json

	  # Delete a pod based on the type and ID in the JSON passed into stdin.
	  cat pod.json | %[1]s delete -f -

	  # Delete pods and services with label name=myLabel.
	  %[1]s delete pods,services -l name=myLabel

	  # Delete a pod with name node-1-vsjnm.
	  %[1]s delete pod node-1-vsjnm

	  # Delete all resources associated with a running app, includes
	  # buildconfig,deploymentconfig,service,imagestream,route and pod,
	  # where 'appName' is listed in 'Labels' of 'oc describe [resource] [resource name]' output.
	  %[1]s delete all -l app=appName

	  # Delete all pods
	  %[1]s delete pods --all`)
)

// NewCmdDelete is a wrapper for the Kubernetes cli delete command
func NewCmdDelete(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdDelete(f, out, errOut)
	cmd.Long = deleteLong
	cmd.Short = "Delete one or more resources"
	cmd.Example = fmt.Sprintf(deleteExample, fullName)
	cmd.SuggestFor = []string{"remove", "stop"}
	return cmd
}

var (
	createLong = templates.LongDesc(`
		Create a resource by filename or stdin

		JSON and YAML formats are accepted.`)

	createExample = templates.Examples(`
		# Create a pod using the data in pod.json.
	  %[1]s create -f pod.json

	  # Create a pod based on the JSON passed into stdin.
	  cat pod.json | %[1]s create -f -`)
)

// NewCmdCreate is a wrapper for the Kubernetes cli create command
func NewCmdCreate(parentName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdCreate(f, out, errOut)
	cmd.Long = createLong
	cmd.Example = fmt.Sprintf(createExample, parentName)

	// normalize long descs and examples
	// TODO remove when normalization is moved upstream
	templates.NormalizeAll(cmd)

	// create subcommands
	cmd.AddCommand(create.NewCmdCreateRoute(parentName, f, out, errOut))
	cmd.AddCommand(create.NewCmdCreatePolicyBinding(create.PolicyBindingRecommendedName, parentName+" create "+create.PolicyBindingRecommendedName, f, out))
	cmd.AddCommand(create.NewCmdCreateDeploymentConfig(create.DeploymentConfigRecommendedName, parentName+" create "+create.DeploymentConfigRecommendedName, f, out))
	cmd.AddCommand(create.NewCmdCreateClusterQuota(create.ClusterQuotaRecommendedName, parentName+" create "+create.ClusterQuotaRecommendedName, f, out))

	cmd.AddCommand(create.NewCmdCreateUser(create.UserRecommendedName, parentName+" create "+create.UserRecommendedName, f, out))
	cmd.AddCommand(create.NewCmdCreateIdentity(create.IdentityRecommendedName, parentName+" create "+create.IdentityRecommendedName, f, out))
	cmd.AddCommand(create.NewCmdCreateUserIdentityMapping(create.UserIdentityMappingRecommendedName, parentName+" create "+create.UserIdentityMappingRecommendedName, f, out))
	cmd.AddCommand(create.NewCmdCreateImageStream(create.ImageStreamRecommendedName, parentName+" create "+create.ImageStreamRecommendedName, f, out))

	adjustCmdExamples(cmd, parentName, "create")

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

func NewCmdCompletion(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmdHelpName := fullName

	if strings.HasSuffix(fullName, "completion") {
		cmdHelpName = "openshift"
	}

	cmd := kcmd.NewCmdCompletion(out, "\n")
	cmd.Long = fmt.Sprintf(completionLong, cmdHelpName)
	cmd.Example = fmt.Sprintf(completionExample, cmdHelpName, cmdHelpName, cmdHelpName, cmdHelpName)
	return cmd
}

var (
	execLong = templates.LongDesc(`Execute a command in a container`)

	execExample = templates.Examples(`
	# Get output from running 'date' in ruby-container from pod 'mypod'
  %[1]s exec mypod -c ruby-container date

  # Switch to raw terminal mode, sends stdin to 'bash' in ruby-container from pod 'mypod' and sends stdout/stderr from 'bash' back to the client
  %[1]s exec mypod -c ruby-container -i -t -- bash -il`)
)

// NewCmdExec is a wrapper for the Kubernetes cli exec command
func NewCmdExec(fullName string, f *clientcmd.Factory, cmdIn io.Reader, cmdOut, cmdErr io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdExec(f, cmdIn, cmdOut, cmdErr)
	cmd.Use = "exec [options] POD [-c CONTAINER] -- COMMAND [args...]"
	cmd.Long = execLong
	cmd.Example = fmt.Sprintf(execExample, fullName)
	cmd.Flag("pod").Usage = cmd.Flag("pod").Usage + " (deprecated)"
	return cmd
}

var (
	portForwardLong = templates.LongDesc(`Forward 1 or more local ports to a pod`)

	portForwardExample = templates.Examples(`
		# Listens on ports 5000 and 6000 locally, forwarding data to/from ports 5000 and 6000 in the pod
	  %[1]s port-forward mypod 5000 6000

	  # Listens on port 8888 locally, forwarding to 5000 in the pod
	  %[1]s port-forward mypod 8888:5000

	  # Listens on a random port locally, forwarding to 5000 in the pod
	  %[1]s port-forward mypod :5000

	  # Listens on a random port locally, forwarding to 5000 in the pod
	  %[1]s port-forward mypod 0:5000`)
)

// NewCmdPortForward is a wrapper for the Kubernetes cli port-forward command
func NewCmdPortForward(fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdPortForward(f, out, errout)
	cmd.Long = portForwardLong
	cmd.Example = fmt.Sprintf(portForwardExample, fullName)
	cmd.Flag("pod").Usage = cmd.Flag("pod").Usage + " (deprecated)"
	return cmd
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
func NewCmdDescribe(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdDescribe(f, out, errOut)
	cmd.Long = describeLong
	cmd.Example = fmt.Sprintf(describeExample, fullName)
	cmd.ValidArgs = describe.DescribableResources()
	return cmd
}

var (
	proxyLong = templates.LongDesc(`Run a proxy to the API server`)

	proxyExample = templates.Examples(`
		# Run a proxy to the api server on port 8011, serving static content from ./local/www/
	  %[1]s proxy --port=8011 --www=./local/www/

	  # Run a proxy to the api server on an arbitrary local port.
	  # The chosen port for the server will be output to stdout.
	  %[1]s proxy --port=0

	  # Run a proxy to the api server, changing the api prefix to my-api
	  # This makes e.g. the pods api available at localhost:8011/my-api/api/v1/pods/
	  %[1]s proxy --api-prefix=/my-api`)
)

// NewCmdProxy is a wrapper for the Kubernetes cli proxy command
func NewCmdProxy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdProxy(f, out)
	cmd.Long = proxyLong
	cmd.Example = fmt.Sprintf(proxyExample, fullName)
	return cmd
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
func NewCmdScale(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdScale(f, out)
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
func NewCmdAutoscale(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdAutoscale(f, out)
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
func NewCmdRun(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	opts := &kcmd.RunOptions{DefaultRestartAlwaysGenerator: "deploymentconfig/v1", DefaultGenerator: kcmdutil.RunPodV1GeneratorName}
	cmd := kcmd.NewCmdRunWithOptions(f, opts, in, out, errout)
	cmd.Long = runLong
	cmd.Example = fmt.Sprintf(runExample, fullName)
	cmd.SuggestFor = []string{"image"}
	cmd.Flags().Set("generator", "")
	cmd.Flag("generator").Usage = "The name of the API generator to use.  Default is 'deploymentconfig/v1' if --restart=Always, otherwise the default is 'run-pod/v1'."
	cmd.Flag("generator").DefValue = ""
	cmd.Flag("generator").Changed = false
	return cmd
}

var (
	attachLong = templates.LongDesc(`
		Attach to a running container

		Attach the current shell to a remote container, returning output or setting up a full
		terminal session. Can be used to debug containers and invoke interactive commands.`)

	attachExample = templates.Examples(`
		# Get output from running pod 123456-7890, using the first container by default
	  %[1]s attach 123456-7890

	  # Get output from ruby-container from pod 123456-7890
	  %[1]s attach 123456-7890 -c ruby-container

	  # Switch to raw terminal mode, sends stdin to 'bash' in ruby-container from pod 123456-780
	  # and sends stdout/stderr from 'bash' back to the client
	  %[1]s attach 123456-7890 -c ruby-container -i -t`)
)

// NewCmdAttach is a wrapper for the Kubernetes cli attach command
func NewCmdAttach(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdAttach(f, in, out, errout)
	cmd.Long = attachLong
	cmd.Example = fmt.Sprintf(attachExample, fullName)
	return cmd
}

var (
	annotateLong = templates.LongDesc(`
		Update the annotations on one or more resources

		An annotation is a key/value pair that can hold larger (compared to a label),
		and possibly not human-readable, data. It is intended to store non-identifying
		auxiliary data, especially data manipulated by tools and system extensions. If
		--overwrite is true, then existing annotations can be overwritten, otherwise
		attempting to overwrite an annotation will result in an error. If
		--resource-version is specified, then updates will use this resource version,
		otherwise the existing resource-version will be used.

		Run '%[1]s types' for a list of valid resources.`)

	annotateExample = templates.Examples(`
		# Update pod 'foo' with the annotation 'description' and the value 'my frontend'.
	  # If the same annotation is set multiple times, only the last value will be applied
	  %[1]s annotate pods foo description='my frontend'

	  # Update pod 'foo' with the annotation 'description' and the value
	  # 'my frontend running nginx', overwriting any existing value.
	  %[1]s annotate --overwrite pods foo description='my frontend running nginx'

	  # Update all pods in the namespace
	  %[1]s annotate pods --all description='my frontend running nginx'

	  # Update pod 'foo' only if the resource is unchanged from version 1.
	  %[1]s annotate pods foo description='my frontend running nginx' --resource-version=1

	  # Update pod 'foo' by removing an annotation named 'description' if it exists.
	  # Does not require the --overwrite flag.
	  %[1]s annotate pods foo description-`)
)

// NewCmdAnnotate is a wrapper for the Kubernetes cli annotate command
func NewCmdAnnotate(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdAnnotate(f, out)
	cmd.Long = fmt.Sprintf(annotateLong, fullName)
	cmd.Example = fmt.Sprintf(annotateExample, fullName)
	return cmd
}

var (
	labelLong = templates.LongDesc(`
		Update the labels on one or more resources

		A valid label value is consisted of letters and/or numbers with a max length of %[1]d
		characters. If --overwrite is true, then existing labels can be overwritten, otherwise
		attempting to overwrite a label will result in an error. If --resource-version is
		specified, then updates will use this resource version, otherwise the existing
		resource-version will be used.`)

	labelExample = templates.Examples(`
		# Update pod 'foo' with the label 'unhealthy' and the value 'true'.
	  %[1]s label pods foo unhealthy=true

	  # Update pod 'foo' with the label 'status' and the value 'unhealthy', overwriting any existing value.
	  %[1]s label --overwrite pods foo status=unhealthy

	  # Update all pods in the namespace
	  %[1]s label pods --all status=unhealthy

	  # Update pod 'foo' only if the resource is unchanged from version 1.
	  %[1]s label pods foo status=unhealthy --resource-version=1

	  # Update pod 'foo' by removing a label named 'bar' if it exists.
	  # Does not require the --overwrite flag.
	  %[1]s label pods foo bar-`)
)

// NewCmdLabel is a wrapper for the Kubernetes cli label command
func NewCmdLabel(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdLabel(f, out)
	cmd.Long = fmt.Sprintf(labelLong, kvalidation.LabelValueMaxLength)
	cmd.Example = fmt.Sprintf(labelExample, fullName)
	return cmd
}

var (
	applyLong = templates.LongDesc(`
		Apply a configuration to a resource by filename or stdin.

		JSON and YAML formats are accepted.`)

	applyExample = templates.Examples(`
		# Apply the configuration in pod.json to a pod.
		%[1]s apply -f ./pod.json

		# Apply the JSON passed into stdin to a pod.
		cat pod.json | %[1]s apply -f -`)
)

// NewCmdApply is a wrapper for the Kubernetes cli apply command
func NewCmdApply(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdApply(f, out, errOut)
	cmd.Long = applyLong
	cmd.Example = fmt.Sprintf(applyExample, fullName)
	return cmd
}

var (
	explainLong = templates.LongDesc(`
		Documentation of resources.

		Possible resource types include: pods (po), services (svc),
		replicationcontrollers (rc), nodes (no), events (ev), componentstatuses (cs),
		limitranges (limits), persistentvolumes (pv), persistentvolumeclaims (pvc),
		resourcequotas (quota), namespaces (ns) or endpoints (ep).`)

	explainExample = templates.Examples(`
		# Get the documentation of the resource and its fields
		%[1]s explain pods

		# Get the documentation of a specific field of a resource
		%[1]s explain pods.spec.containers`)
)

// NewCmdExplain is a wrapper for the Kubernetes cli explain command
func NewCmdExplain(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdExplain(f, out, errOut)
	cmd.Long = explainLong
	cmd.Example = fmt.Sprintf(explainExample, fullName)
	return cmd
}

var (
	convertLong = templates.LongDesc(`
		Convert config files between different API versions. Both YAML
		and JSON formats are accepted.

		The command takes filename, directory, or URL as input, and convert it into format
		of version specified by --output-version flag. If target version is not specified or
		not supported, convert to latest version.

		The default output will be printed to stdout in YAML format. One can use -o option
		to change to output destination.`)

	convertExample = templates.Examples(`
		# Convert 'pod.yaml' to latest version and print to stdout.
	  %[1]s convert -f pod.yaml

	  # Convert the live state of the resource specified by 'pod.yaml' to the latest version
	  # and print to stdout in json format.
	  %[1]s convert -f pod.yaml --local -o json

	  # Convert all files under current directory to latest version and create them all.
	  %[1]s convert -f . | %[1]s create -f -`)
)

// NewCmdConvert is a wrapper for the Kubernetes cli convert command
func NewCmdConvert(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdConvert(f, out)
	cmd.Long = convertLong
	cmd.Example = fmt.Sprintf(convertExample, fullName)
	return cmd
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
func NewCmdEdit(fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdEdit(f, out, errout)
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
func NewCmdConfig(parentName, name string, out, errOut io.Writer) *cobra.Command {
	pathOptions := &kclientcmd.PathOptions{
		GlobalFile:       cmdconfig.RecommendedHomeFile,
		EnvVar:           cmdconfig.OpenShiftConfigPathEnvVar,
		ExplicitFileFlag: cmdconfig.OpenShiftConfigFlagName,

		GlobalFileSubpath: cmdconfig.OpenShiftConfigHomeDirFileName,

		LoadingRules: cmdconfig.NewOpenShiftClientConfigLoadingRules(),
	}
	pathOptions.LoadingRules.DoNotResolvePaths = true

	cmd := config.NewCmdConfig(pathOptions, out, errOut)
	cmd.Short = "Change configuration files for the client"
	cmd.Long = configLong
	cmd.Example = fmt.Sprintf(configExample, parentName, name)
	// normalize long descs and examples
	// TODO remove when normalization is moved upstream
	templates.NormalizeAll(cmd)
	adjustCmdExamples(cmd, parentName, name)
	return cmd
}

var (
	cpExample = templates.Examples(`
	    # !!!Important Note!!!
	    # Requires that the 'tar' binary is present in your container
	    # image.  If 'tar' is not present, 'kubectl cp' will fail.

	    # Copy /tmp/foo_dir local directory to /tmp/bar_dir in a remote pod in the default namespace
		%[1]s cp /tmp/foo_dir <some-pod>:/tmp/bar_dir

        # Copy /tmp/foo local file to /tmp/bar in a remote pod in a specific container
		%[1]s cp /tmp/foo <some-pod>:/tmp/bar -c <specific-container>

		# Copy /tmp/foo local file to /tmp/bar in a remote pod in namespace <some-namespace>
		%[1]s cp /tmp/foo <some-namespace>/<some-pod>:/tmp/bar

		# Copy /tmp/foo from a remote pod to /tmp/bar locally
		%[1]s cp <some-namespace>/<some-pod>:/tmp/foo /tmp/bar`)
)

// NewCmdCp is a wrapper for the Kubernetes cli cp command
func NewCmdCp(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdCp(f, out, errout)
	cmd.Example = fmt.Sprintf(cpExample, fullName)
	return cmd
}

func NewCmdAuth(fullName string, f *clientcmd.Factory, out, errout io.Writer) *cobra.Command {
	cmd := kcmdauth.NewCmdAuth(f, out, errout)
	return cmd
}
