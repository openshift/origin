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
