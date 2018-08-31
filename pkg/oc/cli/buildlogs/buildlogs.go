package buildlogs

import (
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	buildv1 "github.com/openshift/api/build/v1"
	buildclient "github.com/openshift/client-go/build/clientset/versioned"
	buildmanualclient "github.com/openshift/origin/pkg/build/client/v1"
	"github.com/openshift/origin/pkg/oc/cli/logs"
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

type BuildLogsOptions struct {
	Follow bool
	NoWait bool

	Name        string
	Namespace   string
	BuildClient buildclient.Interface

	genericclioptions.IOStreams
}

func NewBuildLogsOptions(streams genericclioptions.IOStreams) *BuildLogsOptions {
	return &BuildLogsOptions{
		IOStreams: streams,
		Follow:    true,
	}
}

// NewCmdBuildLogs implements the OpenShift cli build-logs command
func NewCmdBuildLogs(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewBuildLogsOptions(streams)
	cmd := &cobra.Command{
		Use:        "build-logs BUILD",
		Short:      "Show logs from a build",
		Long:       buildLogsLong,
		Example:    fmt.Sprintf(buildLogsExample, fullName),
		Deprecated: fmt.Sprintf("use oc %v build/<build-name>", logs.LogsRecommendedCommandName),
		Hidden:     true,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.RunBuildLogs())
		},
	}
	cmd.Flags().BoolVarP(&o.Follow, "follow", "f", o.Follow, "Specify whether logs should be followed; default is true.")
	cmd.Flags().BoolVarP(&o.NoWait, "nowait", "w", o.NoWait, "Specify whether to return immediately without waiting for logs to be available; default is false.")

	return cmd
}

func (o *BuildLogsOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("build name is required")
	}
	o.Name = args[0]

	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.BuildClient, err = buildclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return nil
}

// RunBuildLogs contains all the necessary functionality for the OpenShift cli build-logs command
func (o *BuildLogsOptions) RunBuildLogs() error {
	opts := buildv1.BuildLogOptions{
		Follow: o.Follow,
		NoWait: o.NoWait,
	}
	readCloser, err := buildmanualclient.NewBuildLogClient(o.BuildClient.BuildV1().RESTClient(), o.Namespace).Logs(o.Name, opts).Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()

	_, err = io.Copy(o.Out, readCloser)
	if err, ok := err.(errors.APIStatus); ok {
		if err.Status().Code == http.StatusNotFound {
			switch err.Status().Details.Kind {
			case "build":
				return fmt.Errorf("the build %s could not be found, therefore build logs cannot be retrieved", err.Status().Details.Name)
			case "pod":
				return fmt.Errorf("the pod %s for build %s could not be found, therefore build logs cannot be retrieved", err.Status().Details.Name, o.Name)
			}
		}
	}
	return err
}
