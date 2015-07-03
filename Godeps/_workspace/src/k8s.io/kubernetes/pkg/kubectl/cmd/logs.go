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
	"io"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/api"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
)

const (
	logs_example = `// Returns snapshot of ruby-container logs from pod 123456-7890.
$ kubectl logs 123456-7890 -c ruby-container

// Returns snapshot of previous terminated ruby-container logs from pod 123456-7890.
$ kubectl logs -p 123456-7890 -c ruby-container

// Starts streaming of ruby-container logs from pod 123456-7890.
$ kubectl logs -f 123456-7890 -c ruby-container`
)

// NewCmdLogs creates a new pod log command
func NewCmdLogs(f *cmdutil.Factory, out io.Writer) *cobra.Command {
	opts := &api.PodLogOptions{}
	cmd := &cobra.Command{
		Use:     "logs [-f] [-p] POD [-c CONTAINER]",
		Short:   "Print the logs for a container in a pod.",
		Long:    "Print the logs for a container in a pod. If the pod has only one container, the container name is optional.",
		Example: logs_example,
		Run: func(cmd *cobra.Command, args []string) {
			err := RunLogs(f, out, cmd, args, opts)
			cmdutil.CheckErr(err)
		},
		Aliases: []string{"log"},
	}
	cmd.Flags().BoolVarP(&opts.Follow, "follow", "f", false, "Specify if the logs should be streamed.")
	cmd.Flags().BoolVarP(&opts.Previous, "previous", "p", false, "If true, print the logs for the previous instance of the container in a pod if it exists.")
	cmd.Flags().StringVarP(&opts.Container, "container", "c", "", "The container to use for printing its logs")
	return cmd
}

// RunLogs retrieves a pod log
func RunLogs(f *cmdutil.Factory, out io.Writer, cmd *cobra.Command, args []string, opts *api.PodLogOptions) error {
	if len(os.Args) > 1 && os.Args[1] == "log" {
		printDeprecationWarning("logs", "log")
	}

	if len(args) != 1 {
		return cmdutil.UsageError(cmd, "logs [-f] [-p] POD [-c CONTAINER]")
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	r := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		NamespaceParam(namespace).DefaultNamespace().
		ResourceNames("pods", args...).
		SingleResourceType()
	obj, err := r.Do().Object()
	if err != nil {
		return err
	}

	readCloser, err := f.LogsForObject(obj, opts)
	if err != nil {
		return err
	}

	defer readCloser.Close()
	_, err = io.Copy(out, readCloser)
	return err
}
