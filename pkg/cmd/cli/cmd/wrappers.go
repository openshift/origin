package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	kcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	get_long = `Display one or many resources.

Possible resources include builds, buildConfigs, services, pods, etc.`

	get_example = `  // List all pods in ps output format.
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
	cmd := kcmd.NewCmdGet(f.Factory, out)
	cmd.Long = get_long
	cmd.Example = fmt.Sprintf(get_example, fullName)
	return cmd
}

const (
	update_long = `Update a resource by filename or stdin.

JSON and YAML formats are accepted.`

	update_example = `  // Update a pod using the data in pod.json.
  $ %[1]s update -f pod.json

  // Update a pod based on the JSON passed into stdin.
  $ cat pod.json | %[1]s update -f -

  // Update a pod by downloading it, applying the patch, then updating. Requires apiVersion be specified.
  $ %[1]s update pods my-pod --patch='{ "apiVersion": "v1beta1", "desiredState": { "manifest": [{ "cpu": 100 }]}}'`
)

// NewCmdUpdate is a wrapper for the Kubernetes cli update command
func NewCmdUpdate(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdUpdate(f.Factory, out)
	cmd.Long = update_long
	cmd.Example = fmt.Sprintf(update_example, fullName)
	return cmd
}

const (
	delete_long = `Delete a resource by filename, stdin, resource and ID, or by resources and label selector.

JSON and YAML formats are accepted.

If both a filename and command line arguments are passed, the command line
arguments are used and the filename is ignored.

Note that the delete command does NOT do resource version checks, so if someone
submits an update to a resource right when you submit a delete, their update
will be lost along with the rest of the resource.`

	delete_example = `  // Delete a pod using the type and ID specified in pod.json.
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
	cmd.Long = delete_long
	cmd.Example = fmt.Sprintf(delete_example, fullName)
	return cmd
}

const (
	log_long = `Print the logs for a container in a pod. If the pod has only one container, the container name is optional.`

	log_example = `  // Returns snapshot of ruby-container logs from pod 123456-7890.
  $ %[1]s log 123456-7890 ruby-container

  // Starts streaming of ruby-container logs from pod 123456-7890.
  $ %[1]s log -f 123456-7890 ruby-container`
)

// NewCmdLog is a wrapper for the Kubernetes cli log command
func NewCmdLog(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdLog(f.Factory, out)
	cmd.Long = log_long
	cmd.Example = fmt.Sprintf(log_example, fullName)
	return cmd
}

const (
	create_long = `Create a resource by filename or stdin.

JSON and YAML formats are accepted.`

	create_example = `  // Create a pod using the data in pod.json.
  $ %[1]s create -f pod.json

  // Create a pod based on the JSON passed into stdin.
  $ cat pod.json | %[1]s create -f -`
)

// NewCmdCreate is a wrapper for the Kubernetes cli create command
func NewCmdCreate(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdCreate(f.Factory, out)
	cmd.Long = create_long
	cmd.Example = fmt.Sprintf(create_example, fullName)
	return cmd
}

const (
	exec_long = `Execute a command in a container.`

	exec_example = `  // Get output from running 'date' in ruby-container from pod 123456-7890
  $ %[1]s exec -p 123456-7890 -c ruby-container date

  // Switch to raw terminal mode, sends stdin to 'bash' in ruby-container from pod 123456-780 and sends stdout/stderr from 'bash' back to the client
  $ %[1]s exec -p 123456-7890 -c ruby-container -i -t -- bash -il`
)

// NewCmdExec is a wrapper for the Kubernetes cli exec command
func NewCmdExec(fullName string, f *clientcmd.Factory, cmdIn io.Reader, cmdOut, cmdErr io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdExec(f.Factory, cmdIn, cmdOut, cmdErr)
	cmd.Long = exec_long
	cmd.Example = fmt.Sprintf(exec_example, fullName)
	return cmd
}

const (
	portForward_long = `Forward 1 or more local ports to a pod.`

	portForward_example = `  // Listens on ports 5000 and 6000 locally, forwarding data to/from ports 5000 and 6000 in the pod
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
	cmd.Long = portForward_long
	cmd.Example = fmt.Sprintf(portForward_example, fullName)
	return cmd
}

const (
	describe_long = `Show details of a specific resource.

This command joins many API calls together to form a detailed description of a
given resource.`

	describe_example = `  // Provide details about the ruby-20-centos7 image repository
  $ %[1]s describe imageRepository ruby-20-centos7

  // Provide details about the ruby-sample-build build configuration
  $ %[1]s describe bc ruby-sample-build`
)

// NewCmdDescribe is a wrapper for the Kubernetes cli describe command
func NewCmdDescribe(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdDescribe(f.Factory, out)
	cmd.Long = describe_long
	cmd.Example = fmt.Sprintf(describe_example, fullName)
	return cmd
}

const (
	proxy_long = `Run a proxy to the Kubernetes API server.`

	proxy_example = `  // Run a proxy to kubernetes apiserver on port 8011, serving static content from ./local/www/
  $ %[1]s proxy --port=8011 --www=./local/www/

  // Run a proxy to kubernetes apiserver, changing the api prefix to k8s-api
  // This makes e.g. the pods api available at localhost:8011/k8s-api/v1beta1/pods/
  $ %[1]s proxy --api-prefix=k8s-api`
)

// NewCmdProxy is a wrapper for the Kubernetes cli proxy command
func NewCmdProxy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := kcmd.NewCmdProxy(f.Factory, out)
	cmd.Long = proxy_long
	cmd.Example = fmt.Sprintf(proxy_example, fullName)
	return cmd
}

func tab(original string) string {
	lines := []string{}
	scanner := bufio.NewScanner(strings.NewReader(original))
	for scanner.Scan() {
		lines = append(lines, "  "+scanner.Text())
	}
	return strings.Join(lines, "\n")
}
