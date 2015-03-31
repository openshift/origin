package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

func NewCmdGet(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := f.NewCmdGet(out)
	longDesc := `Display one or many resources.

Possible resources include builds, buildConfigs, services, pods, etc.

Examples:

	# List all pods in ps output format.
	$ %[1]s get pods

	# List a single replication controller with specified ID in ps output format.
	$ %[1]s get replicationController 1234-56-7890-234234-456456

	# List a single pod in JSON output format.
	$ %[1]s get -o json pod 1234-56-7890-234234-456456

	# Return only the status value of the specified pod.
	$ %[1]s get -o template pod 1234-56-7890-234234-456456 --template={{.currentState.status}}
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}

func NewCmdUpdate(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := f.NewCmdUpdate(out)
	longDesc := `Update a resource by filename or stdin.

JSON and YAML formats are accepted.

Examples:

	# Update a pod using the data in pod.json.
	$ %[1]s update -f pod.json

	# Update a pod based on the JSON passed into stdin.
	$ cat pod.json | %[1]s update -f -

	# Update a pod by downloading it, applying the patch, then updating. Requires apiVersion be specified.
	$ %[1]s update pods my-pod --patch='{ "apiVersion": "v1beta1", "desiredState": { "manifest": [{ "cpu": 100 }]}}'
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}

func NewCmdDelete(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := f.NewCmdDelete(out)
	longDesc := `Delete a resource by filename, stdin, resource and ID, or by resources and label selector.

JSON and YAML formats are accepted.

If both a filename and command line arguments are passed, the command line
arguments are used and the filename is ignored.

Note that the delete command does NOT do resource version checks, so if someone
submits an update to a resource right when you submit a delete, their update
will be lost along with the rest of the resource.

Examples:

	# Delete a pod using the type and ID specified in pod.json.
	$ %[1]s delete -f pod.json

	# Delete a pod based on the type and ID in the JSON passed into stdin.
	$ cat pod.json | %[1]s delete -f -

	# Delete pods and services with label name=myLabel.
	$ %[1]s delete pods,services -l name=myLabel

	# Delete a pod with ID 1234-56-7890-234234-456456.
	$ %[1]s delete pod 1234-56-7890-234234-456456

	# Delete all pods
	$ %[1]s delete pods --all
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}

func NewCmdLog(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := f.NewCmdLog(out)
	longDesc := `Print the logs for a container in a pod. If the pod has only one container, the container name is optional.

Examples:

	# Returns snapshot of ruby-container logs from pod 123456-7890.
	$ %[1]s log 123456-7890 ruby-container

	# Starts streaming of ruby-container logs from pod 123456-7890.
	$ %[1]s log -f 123456-7890 ruby-container
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}

func NewCmdCreate(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := f.NewCmdCreate(out)
	longDesc := `Create a resource by filename or stdin.

JSON and YAML formats are accepted.

Examples:

	# Create a pod using the data in pod.json.
	$ %[1]s create -f pod.json

	# Create a pod based on the JSON passed into stdin.
	$ cat pod.json | %[1]s create -f -
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}

func NewCmdExec(fullName string, f *clientcmd.Factory, cmdIn io.Reader, cmdOut, cmdErr io.Writer) *cobra.Command {
	cmd := f.NewCmdExec(cmdIn, cmdOut, cmdErr)
	longDesc := `Execute a command in a container.

Examples:

	# get output from running 'date' in ruby-container from pod 123456-7890
	$ %[1]s exec -p 123456-7890 -c ruby-container date

	# switch to raw terminal mode, sends stdin to 'bash' in ruby-container from pod 123456-780 and sends stdout/stderr from 'bash' back to the client
	$ %[1]s exec -p 123456-7890 -c ruby-container -i -t -- bash -il
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}

func NewCmdPortForward(fullName string, f *clientcmd.Factory) *cobra.Command {
	cmd := f.NewCmdPortForward()
	longDesc := `Forward 1 or more local ports to a pod.

Examples:

	# listens on ports 5000 and 6000 locally, forwarding data to/from ports 5000 and 6000 in the pod
	$ %[1]s port-forward -p mypod 5000 6000

	# listens on port 8888 locally, forwarding to 5000 in the pod
	$ %[1]s port-forward -p mypod 8888:5000

	# listens on a random port locally, forwarding to 5000 in the pod
	$ %[1]s port-forward -p mypod :5000

	# listens on a random port locally, forwarding to 5000 in the pod
	$ %[1]s port-forward -p mypod 0:5000
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}

func NewCmdDescribe(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := f.NewCmdDescribe(out)
	longDesc := `Show details of a specific resource.

This command joins many API calls together to form a detailed description of a
given resource.

Examples:

	# Provide details about the ruby-20-centos7 image repository
	$ %[1]s describe imageRepository ruby-20-centos7

	# Provide details about the ruby-sample-build build configuration
	$ %[1]s describe bc ruby-sample-build
`
	cmd.Long = fmt.Sprintf(longDesc, fullName)
	return cmd
}
