package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	kcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func tab(original string) string {
	lines := []string{}
	scanner := bufio.NewScanner(strings.NewReader(original))
	for scanner.Scan() {
		lines = append(lines, "  "+scanner.Text())
	}
	return strings.Join(lines, "\n")
}

const (
	getLong = `Display one or many resources.

Possible resources include builds, buildConfigs, services, pods, etc.`

	getExample = `  // List all pods in ps output format.
  $ %[1]s get pods

  // List a single replication controller with specified ID in ps output format.
  $ %[1]s get replicationController 1234-56-7890-234234-456456

  // List a single pod in JSON output format.
  $ %[1]s get -o json pod 1234-56-7890-234234-456456

  // Return only the status value of the specified pod.
  $ %[1]s get -o template pod 1234-56-7890-234234-456456 --template={{.currentState.status}}`
)

// NewCmdGet is a wrapper for the Kubernetes cli get command
func NewCmdGet(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	p := describe.NewHumanReadablePrinter(false, false)
	validArgs := p.HandledResources()

	cmd := kcmd.NewCmdGet(f.Factory, out)
	cmd.Long = getLong
	cmd.Example = fmt.Sprintf(getExample, fullName)
	cmd.ValidArgs = validArgs
	return cmd
}

const (
	updateLong = `Update a resource by filename or stdin.

JSON and YAML formats are accepted.`

	updateExample = `  // Update a pod using the data in pod.json.
  $ %[1]s update -f pod.json

  // Update a pod based on the JSON passed into stdin.
  $ cat pod.json | %[1]s update -f -`
)

// NewCmdUpdate is a wrapper for the Kubernetes cli update command
func NewCmdUpdate(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdUpdate(f.Factory, out)
	cmd.Long = updateLong
	cmd.Example = fmt.Sprintf(updateExample, fullName)
	return cmd
}

const (
	deleteLong = `Delete a resource by filename, stdin, resource and ID, or by resources and label selector.

JSON and YAML formats are accepted.

If both a filename and command line arguments are passed, the command line
arguments are used and the filename is ignored.

Note that the delete command does NOT do resource version checks, so if someone
submits an update to a resource right when you submit a delete, their update
will be lost along with the rest of the resource.`

	deleteExample = `  // Delete a pod using the type and ID specified in pod.json.
  $ %[1]s delete -f pod.json

  // Delete a pod based on the type and ID in the JSON passed into stdin.
  $ cat pod.json | %[1]s delete -f -

  // Delete pods and services with label name=myLabel.
  $ %[1]s delete pods,services -l name=myLabel

  // Delete a pod with ID 1234-56-7890-234234-456456.
  $ %[1]s delete pod 1234-56-7890-234234-456456

  // Delete all pods
  $ %[1]s delete pods --all`
)

// NewCmdDelete is a wrapper for the Kubernetes cli delete command
func NewCmdDelete(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdDelete(f.Factory, out)
	cmd.Long = deleteLong
	cmd.Example = fmt.Sprintf(deleteExample, fullName)
	return cmd
}

const (
	logsLong = `Print the logs for a container in a pod. If the pod has only one container, the container name is optional.`

	logsExample = `  // Returns snapshot of ruby-container logs from pod 123456-7890.
  $ %[1]s logs 123456-7890 ruby-container

  // Starts streaming of ruby-container logs from pod 123456-7890.
  $ %[1]s logs -f 123456-7890 ruby-container`
)

// NewCmdLogs is a wrapper for the Kubernetes cli logs command
func NewCmdLogs(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdLog(f.Factory, out)
	cmd.Long = logsLong
	cmd.Example = fmt.Sprintf(logsExample, fullName)
	return cmd
}

const (
	createLong = `Create a resource by filename or stdin.

JSON and YAML formats are accepted.`

	createExample = `  // Create a pod using the data in pod.json.
  $ %[1]s create -f pod.json

  // Create a pod based on the JSON passed into stdin.
  $ cat pod.json | %[1]s create -f -`
)

// NewCmdCreate is a wrapper for the Kubernetes cli create command
func NewCmdCreate(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdCreate(f.Factory, out)
	cmd.Long = createLong
	cmd.Example = fmt.Sprintf(createExample, fullName)
	return cmd
}

const (
	execLong = `Execute a command in a container.`

	execExample = `  // Get output from running 'date' in ruby-container from pod 123456-7890
  $ %[1]s exec -p 123456-7890 -c ruby-container date

  // Switch to raw terminal mode, sends stdin to 'bash' in ruby-container from pod 123456-780 and sends stdout/stderr from 'bash' back to the client
  $ %[1]s exec -p 123456-7890 -c ruby-container -i -t -- bash -il`
)

// NewCmdExec is a wrapper for the Kubernetes cli exec command
func NewCmdExec(fullName string, f *clientcmd.Factory, cmdIn io.Reader, cmdOut, cmdErr io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdExec(f.Factory, cmdIn, cmdOut, cmdErr)
	cmd.Long = execLong
	cmd.Example = fmt.Sprintf(execExample, fullName)
	return cmd
}

const (
	portForwardLong = `Forward 1 or more local ports to a pod.`

	portForwardExample = `  // Listens on ports 5000 and 6000 locally, forwarding data to/from ports 5000 and 6000 in the pod
  $ %[1]s port-forward -p mypod 5000 6000

  // Listens on port 8888 locally, forwarding to 5000 in the pod
  $ %[1]s port-forward -p mypod 8888:5000

  // Listens on a random port locally, forwarding to 5000 in the pod
  $ %[1]s port-forward -p mypod :5000

  // Listens on a random port locally, forwarding to 5000 in the pod
  $ %[1]s port-forward -p mypod 0:5000`
)

// NewCmdPortForward is a wrapper for the Kubernetes cli port-forward command
func NewCmdPortForward(fullName string, f *clientcmd.Factory) *cobra.Command {
	cmd := kcmd.NewCmdPortForward(f.Factory)
	cmd.Long = portForwardLong
	cmd.Example = fmt.Sprintf(portForwardExample, fullName)
	return cmd
}

const (
	describeLong = `Show details of a specific resource.

This command joins many API calls together to form a detailed description of a
given resource.`

	describeExample = `  // Provide details about the ruby-20-centos7 image repository
  $ %[1]s describe imageRepository ruby-20-centos7

  // Provide details about the ruby-sample-build build configuration
  $ %[1]s describe bc ruby-sample-build`
)

// NewCmdDescribe is a wrapper for the Kubernetes cli describe command
func NewCmdDescribe(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdDescribe(f.Factory, out)
	cmd.Long = describeLong
	cmd.Example = fmt.Sprintf(describeExample, fullName)
	cmd.ValidArgs = describe.DescribableResources()
	return cmd
}

const (
	proxyLong = `Run a proxy to the Kubernetes API server.`

	proxyExample = `  // Run a proxy to kubernetes apiserver on port 8011, serving static content from ./local/www/
  $ %[1]s proxy --port=8011 --www=./local/www/

  // Run a proxy to kubernetes apiserver, changing the api prefix to k8s-api
  // This makes e.g. the pods api available at localhost:8011/k8s-api/v1beta3/pods/
  $ %[1]s proxy --api-prefix=k8s-api`
)

// NewCmdProxy is a wrapper for the Kubernetes cli proxy command
func NewCmdProxy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdProxy(f.Factory, out)
	cmd.Long = proxyLong
	cmd.Example = fmt.Sprintf(proxyExample, fullName)
	return cmd
}

const (
	scaleLong = `Set a new size for a Replication Controller either directly or via its Deployment Configuration.

Scale also allows users to specify one or more preconditions for the scale action.
If --current-replicas or --resource-version is specified, it is validated before the
scale is attempted, and it is guaranteed that the precondition holds true when the
scale is sent to the server.`
	scaleExample = `  // Scale replication controller named 'foo' to 3.
  $ %[1]s scale --replicas=3 replicationcontrollers foo

  // If the replication controller named foo's current size is 2, scale foo to 3.
  $ %[1]s scale --current-replicas=2 --replicas=3 replicationcontrollers foo`
)

// NewCmdScale is a wrapper for the Kubernetes cli scale command
func NewCmdScale(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdScale(f.Factory, out)
	cmd.Short = "Change the number of pods in a deployment"
	cmd.Long = scaleLong
	cmd.Example = fmt.Sprintf(scaleExample, fullName)
	return cmd
}

const (
	stopLong = `Gracefully shut down a resource by id or filename.

Attempts to shut down and delete a resource that supports graceful termination.
If the resource is scalable it will be scaled to 0 before deletion.`

	stopExample = `  // Shut down foo.
  $ %[1]s stop replicationcontroller foo

  // Stop pods and services with label name=myLabel.
  $ %[1]s stop pods,services -l name=myLabel

  // Shut down the service defined in service.json
  $ %[1]s stop -f service.json

  // Shut down all resources in the path/to/resources directory
  $ %[1]s stop -f path/to/resources`
)

// NewCmdStop is a wrapper for the Kubernetes cli stop command
func NewCmdStop(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdStop(f.Factory, out)
	cmd.Long = stopLong
	cmd.Example = fmt.Sprintf(stopExample, fullName)
	return cmd
}

const (
	labelLong = `Update the labels on a resource.

A valid label value is consisted of letters and/or numbers with a max length of %[1]d characters.
If --overwrite is true, then existing labels can be overwritten, otherwise attempting to overwrite a label will result in an error.
If --resource-version is specified, then updates will use this resource version, otherwise the existing resource-version will be used.`

	labelExample = `  // Update pod 'foo' with the label 'unhealthy' and the value 'true'.
  $ %[1]s label pods foo unhealthy=true

  // Update pod 'foo' with the label 'status' and the value 'unhealthy', overwriting any existing value.
  $ %[1]s label --overwrite pods foo status=unhealthy

  // Update all pods in the namespace
  $ %[1]s label pods --all status=unhealthy

  // Update pod 'foo' only if the resource is unchanged from version 1.
  $ %[1]s label pods foo status=unhealthy --resource-version=1

  // Update pod 'foo' by removing a label named 'bar' if it exists.
  // Does not require the --overwrite flag.
  $ %[1]s label pods foo bar-`
)

// NewCmdLabel is a wrapper for the Kubernetes cli label command
func NewCmdLabel(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdLabel(f.Factory, out)
	cmd.Long = fmt.Sprintf(labelLong, kutil.LabelValueMaxLength)
	cmd.Example = fmt.Sprintf(labelExample, fullName)
	return cmd
}
