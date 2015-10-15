/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util/sets"
)

const (
	log_example = `# Return snapshot of ruby-container logs from pod 123456-7890.
$ kubectl logs 123456-7890 ruby-container

# Return snapshot of previous terminated ruby-container logs from pod 123456-7890.
$ kubectl logs -p 123456-7890 ruby-container

# Start streaming of ruby-container logs from pod 123456-7890.
$ kubectl logs -f 123456-7890 ruby-container`
)

func selectContainer(pod *api.Pod, in io.Reader, out io.Writer) string {
	fmt.Fprintf(out, "Please select a container:\n")
	options := sets.String{}
	for ix := range pod.Spec.Containers {
		fmt.Fprintf(out, "[%d] %s\n", ix+1, pod.Spec.Containers[ix].Name)
		options.Insert(pod.Spec.Containers[ix].Name)
	}
	for {
		var input string
		fmt.Fprintf(out, "> ")
		fmt.Fscanln(in, &input)
		if options.Has(input) {
			return input
		}
		ix, err := strconv.Atoi(input)
		if err == nil && ix > 0 && ix <= len(pod.Spec.Containers) {
			return pod.Spec.Containers[ix-1].Name
		}
		fmt.Fprintf(out, "Invalid input: %s", input)
	}
}

type LogsOptions struct {
	Client *client.Client

	PodNamespace  string
	PodName       string
	ContainerName string
	Follow        bool
	Interactive   bool
	Previous      bool
	Out           io.Writer
}

// NewCmdLog creates a new pod log command
func NewCmdLog(f *cmdutil.Factory, out io.Writer) *cobra.Command {
	o := &LogsOptions{
		Out:         out,
		Interactive: true,
	}

	cmd := &cobra.Command{
		Use:     "logs [-f] [-p] POD [-c CONTAINER]",
		Short:   "Print the logs for a container in a pod.",
		Long:    "Print the logs for a container in a pod. If the pod has only one container, the container name is optional.",
		Example: log_example,
		Run: func(cmd *cobra.Command, args []string) {
			if len(os.Args) > 1 && os.Args[1] == "log" {
				printDeprecationWarning("logs", "log")
			}

			cmdutil.CheckErr(o.Complete(f, out, cmd, args))

			cmdutil.CheckErr(o.Validate(cmd))

			cmdutil.CheckErr(o.RunLog())
		},
		Aliases: []string{"log"},
	}
	cmd.Flags().BoolVarP(&o.Follow, "follow", "f", o.Follow, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVar(&o.Interactive, "interactive", o.Interactive, "If true, prompt the user for input when required. Default true.")
	cmd.Flags().BoolVarP(&o.Previous, "previous", "p", o.Previous, "If true, print the logs for the previous instance of the container in a pod if it exists.")
	cmd.Flags().StringVarP(&o.ContainerName, "container", "c", o.ContainerName, "Container name")
	return cmd
}

func (o *LogsOptions) Complete(f *cmdutil.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	switch len(args) {
	case 0:
		return cmdutil.UsageError(cmd, "POD is required for log")

	case 1:
		o.PodName = args[0]
	case 2:
		o.PodName = args[0]
		o.ContainerName = args[1]

	default:
		return cmdutil.UsageError(cmd, "log POD [CONTAINER]")
	}

	var err error
	o.PodNamespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Client, err = f.Client()
	if err != nil {
		return err
	}

	return nil
}

func (o *LogsOptions) Validate(cmd *cobra.Command) error {
	return nil
}

// RunLog retrieves a pod log
func (o *LogsOptions) RunLog() error {
	pod, err := o.Client.Pods(o.PodNamespace).Get(o.PodName)
	if err != nil {
		return err
	}

	// [-c CONTAINER]
	container := o.ContainerName
	if len(container) == 0 {
		// [CONTAINER] (container as arg not flag) is supported as legacy behavior. See PR #10519 for more details.
		if len(pod.Spec.Containers) != 1 {
			podContainersNames := []string{}
			for _, container := range pod.Spec.Containers {
				podContainersNames = append(podContainersNames, container.Name)
			}

			return fmt.Errorf("Pod %s has the following containers: %s; please specify the container to print logs for with -c", pod.ObjectMeta.Name, strings.Join(podContainersNames, ", "))
		}
		container = pod.Spec.Containers[0].Name
	}

	return handleLog(o.Client, o.PodNamespace, o.PodName, container, o.Follow, o.Previous, o.Out)
}

func handleLog(client *client.Client, namespace, podID, container string, follow, previous bool, out io.Writer) error {
	readCloser, err := client.RESTClient.Get().
		Namespace(namespace).
		Name(podID).
		Resource("pods").
		SubResource("log").
		Param("follow", strconv.FormatBool(follow)).
		Param("container", container).
		Param("previous", strconv.FormatBool(previous)).
		Stream()
	if err != nil {
		return err
	}

	defer readCloser.Close()
	_, err = io.Copy(out, readCloser)
	return err
}
