package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/api/errors"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

var (
	buildLogsLong = templates.LongDesc(`
		Retrieve logs for a build

		This command displays the log for the provided build. If the pod that ran the build has been deleted logs
		will no longer be available. If the build has not yet completed, the build logs will be streamed until the
		build completes or fails.`)

	buildLogsExample = templates.Examples(`
		# Stream logs from container
  	%[1]s build-logs 566bed879d2d`)
)

// NewCmdBuildLogs implements the OpenShift cli build-logs command
func NewCmdBuildLogs(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := api.BuildLogOptions{}
	cmd := &cobra.Command{
		Use:        "build-logs BUILD",
		Short:      "Show logs from a build",
		Long:       buildLogsLong,
		Example:    fmt.Sprintf(buildLogsExample, fullName),
		Deprecated: fmt.Sprintf("use \"oc %v build/<build-name>\" instead.", LogsRecommendedCommandName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunBuildLogs(fullName, f, out, cmd, opts, args)

			if err, ok := err.(errors.APIStatus); ok {
				if msg := err.Status().Message; strings.HasSuffix(msg, buildutil.NoBuildLogsMessage) {
					fmt.Fprintf(out, msg)
					os.Exit(1)
				}
				if err.Status().Code == http.StatusNotFound {
					switch err.Status().Details.Kind {
					case "build":
						fmt.Fprintf(out, "The build %s could not be found.  Therefore build logs cannot be retrieved.\n", err.Status().Details.Name)
					case "pod":
						fmt.Fprintf(out, "The pod %s for build %s could not be found.  Therefore build logs cannot be retrieved.\n", err.Status().Details.Name, args[0])
					}
					os.Exit(1)
				}
			}
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().BoolVarP(&opts.Follow, "follow", "f", true, "Specify whether logs should be followed; default is true.")
	cmd.Flags().BoolVarP(&opts.NoWait, "nowait", "w", false, "Specify whether to return immediately without waiting for logs to be available; default is false.")
	return cmd
}

// RunBuildLogs contains all the necessary functionality for the OpenShift cli build-logs command
func RunBuildLogs(fullName string, f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, opts api.BuildLogOptions, args []string) error {
	if len(args) != 1 {
		cmdNamespace := kcmdutil.GetFlagString(cmd, "namespace")
		var namespace string
		if cmdNamespace != "" {
			namespace = " -n " + cmdNamespace
		}
		return kcmdutil.UsageError(cmd, "A build name is required - you can run `%s get builds%s` to list builds", fullName, namespace)
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
