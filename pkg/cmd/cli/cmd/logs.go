package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

const (
	logsLong = `
Print the logs for a resource.

If the pod has only one container, the container name is optional.`

	logsExample = `  # Returns snapshot of ruby-container logs from pod backend.
  $ %[1]s logs backend -c ruby-container

  # Starts streaming of ruby-container logs from pod backend.
  $ %[1]s logs -f pod/backend -c ruby-container

  # Starts streaming the logs of the most recent build of the openldap buildConfig.
  $ %[1]s logs -f bc/openldap

  # Starts streaming the logs of the latest deployment of the mysql deploymentConfig
  $ %[1]s logs -f dc/mysql`
)

// OpenShiftLogsOptions holds all the necessary options for running oc logs.
type OpenShiftLogsOptions struct {
	// Options should hold our own *LogOptions objects.
	Options runtime.Object
	// KubeLogOptions contains all the necessary options for
	// running the upstream logs command.
	KubeLogOptions *kcmd.LogsOptions
}

// NewCmdLogs creates a new logs command that supports OpenShift resources.
func NewCmdLogs(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := OpenShiftLogsOptions{
		KubeLogOptions: &kcmd.LogsOptions{},
	}
	cmd := kcmd.NewCmdLog(f.Factory, out)
	cmd.Short = "Print the logs for a resource."
	cmd.Long = logsLong
	cmd.Example = fmt.Sprintf(logsExample, fullName)
	cmd.Run = func(cmd *cobra.Command, args []string) {
		cmdutil.CheckErr(o.Complete(f, out, cmd, args))
		if err := o.Validate(); err != nil {
			cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
		}
		cmdutil.CheckErr(o.RunLog())
	}

	return cmd
}

// Complete calls the upstream Complete for the logs command and then resolves the
// resource a user requested to view its logs and creates the appropriate logOptions
// object for it.
func (o *OpenShiftLogsOptions) Complete(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string) error {
	if err := o.KubeLogOptions.Complete(f.Factory, out, cmd, args); err != nil {
		return err
	}
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	podLogOptions := o.KubeLogOptions.Options.(*kapi.PodLogOptions)

	mapper, typer := f.Object()
	infos, err := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		NamespaceParam(namespace).DefaultNamespace().
		ResourceNames("pods", args...).
		SingleResourceType().RequireObject(false).
		Do().Infos()
	if err != nil {
		return err
	}
	if len(infos) != 1 {
		return errors.New("expected a resource")
	}
	_, resource := meta.KindToResource(infos[0].Mapping.Kind, false)

	// TODO: podLogOptions should be included in our own logOptions objects.
	switch resource {
	case "build", "buildconfig":
		o.Options = &buildapi.BuildLogOptions{
			Follow:       podLogOptions.Follow,
			SinceSeconds: podLogOptions.SinceSeconds,
			SinceTime:    podLogOptions.SinceTime,
			Timestamps:   podLogOptions.Timestamps,
			TailLines:    podLogOptions.TailLines,
			LimitBytes:   podLogOptions.LimitBytes,
		}
	case "deploymentconfig":
		o.Options = &deployapi.DeploymentLogOptions{
			Follow:       podLogOptions.Follow,
			SinceSeconds: podLogOptions.SinceSeconds,
			SinceTime:    podLogOptions.SinceTime,
			Timestamps:   podLogOptions.Timestamps,
			TailLines:    podLogOptions.TailLines,
			LimitBytes:   podLogOptions.LimitBytes,
		}
	default:
		o.Options = nil
	}

	return nil
}

// Validate runs the upstream validation for the logs command and then it
// will validate any OpenShift-specific log options.
func (o OpenShiftLogsOptions) Validate() error {
	if err := o.KubeLogOptions.Validate(); err != nil {
		return err
	}
	if o.Options == nil {
		return nil
	}
	// TODO: Validate our own options.
	return nil
}

// RunLog will run the upstream logs command and may use an OpenShift
// logOptions object.
func (o OpenShiftLogsOptions) RunLog() error {
	if o.Options != nil {
		// Use our own options object.
		o.KubeLogOptions.Options = o.Options
	}
	_, err := o.KubeLogOptions.RunLog()
	return err
}
