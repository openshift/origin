package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/pkg/units"
	"github.com/spf13/cobra"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	buildLogsLong = `
Retrieve logs for a build

This command displays the log for the provided build. If the pod that ran the build has been deleted logs
will no longer be available. If the build has not yet completed, the build logs will be streamed until the
build completes or fails.`

	buildLogsExample = `  // Stream logs from container
  $ %[1]s build-logs 566bed879d2d`
)

// NewCmdBuildLogs implements the OpenShift cli build-logs command
func NewCmdBuildLogs(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := api.BuildLogOptions{}
	cmd := &cobra.Command{
		Use:     "build-logs BUILD",
		Short:   "Show logs from a build",
		Long:    buildLogsLong,
		Example: fmt.Sprintf(buildLogsExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunBuildLogs(f, out, cmd, opts, args)

			if err, ok := err.(kclient.APIStatus); ok {
				if msg := err.Status().Message; strings.HasSuffix(msg, buildutil.NoBuildLogsMessage) {
					fmt.Fprintf(out, msg)
					os.Exit(1)
				}
			}
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&opts.Follow, "follow", "f", true, "Specify whether logs should be followed; default is true.")
	cmd.Flags().BoolVarP(&opts.NoWait, "nowait", "w", false, "Specify whether to return immediately without waiting for logs to be available; default is false.")
	return cmd
}

// RunBuildLogs contains all the necessary functionality for the OpenShift cli build-logs command
func RunBuildLogs(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, opts api.BuildLogOptions, args []string) error {
	if len(args) != 1 {
		// maximum time to wait for a list of builds
		timeout := 800 * time.Millisecond
		// maximum number of builds to list
		maxBuildListLen := 10
		ch := make(chan error)
		go func() {
			// TODO fetch via API no more than maxBuildListLen builds
			builds, err := getBuilds(f)
			if err != nil {
				return
			}
			if len(builds) == 0 {
				ch <- cmdutil.UsageError(cmd, "There are no builds in the current project")
				return
			}
			sort.Sort(sort.Reverse(api.ByCreationTimestamp(builds)))
			msg := "A build name is required. Most recent builds:"
			for i, b := range builds {
				if i == maxBuildListLen {
					break
				}
				msg += fmt.Sprintf("\n* %s\t%s\t%s ago", b.Name, b.Status.Phase, units.HumanDuration(time.Since(b.CreationTimestamp.Time)))
			}
			ch <- cmdutil.UsageError(cmd, msg)
		}()
		select {
		case <-time.After(timeout):
			return cmdutil.UsageError(cmd, "A build name is required")
		case err := <-ch:
			return err
		}
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	c, _, err := f.Clients()
	if err != nil {
		return err
	}

	readCloser, err := c.BuildLogs(namespace).Get(args[0], opts).Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = io.Copy(out, readCloser)
	return err
}

func getBuilds(f *clientcmd.Factory) ([]api.Build, error) {
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return nil, err
	}

	c, _, err := f.Clients()
	if err != nil {
		return nil, err
	}

	b, err := c.Builds(namespace).List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}

	return b.Items, nil
}
