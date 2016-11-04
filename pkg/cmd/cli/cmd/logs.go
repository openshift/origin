package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// LogsRecommendedCommandName is the recommended command name
// TODO: Probably move this pattern upstream?
const LogsRecommendedCommandName = "logs"

var (
	logsLong = templates.LongDesc(`
		Print the logs for a resource

		Supported resources are builds, build configs (bc), deployment configs (dc), and pods.
		When a pod is specified and has more than one container, the container name should be
		specified via -c. When a build config or deployment config is specified, you can view
		the logs for a particular version of it via --version.

		If your pod is failing to start, you may need to use the --previous option to see the
		logs of the last attempt.`)

	logsExample = templates.Examples(`
		# Start streaming the logs of the most recent build of the openldap build config.
	  %[1]s %[2]s -f bc/openldap

	  # Start streaming the logs of the latest deployment of the mysql deployment config.
	  %[1]s %[2]s -f dc/mysql

	  # Get the logs of the first deployment for the mysql deployment config. Note that logs
	  # from older deployments may not exist either because the deployment was successful
	  # or due to deployment pruning or manual deletion of the deployment.
	  %[1]s %[2]s --version=1 dc/mysql

	  # Return a snapshot of ruby-container logs from pod backend.
	  %[1]s %[2]s backend -c ruby-container

	  # Start streaming of ruby-container logs from pod backend.
	  %[1]s %[2]s -f pod/backend -c ruby-container`)
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
func NewCmdLogs(name, baseName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := OpenShiftLogsOptions{
		KubeLogOptions: &kcmd.LogsOptions{},
	}

	cmd := kcmd.NewCmdLogs(f.Factory, out)
	cmd.Short = "Print the logs for a resource"
	cmd.Long = logsLong
	cmd.Example = fmt.Sprintf(logsExample, baseName, name)
	cmd.SuggestFor = []string{"builds", "deployments"}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		kcmdutil.CheckErr(o.Complete(f, cmd, args, out))

		if err := o.Validate(); err != nil {
			kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
		}

		kcmdutil.CheckErr(o.RunLog())
	}

	cmd.Flags().Int64("version", 0, "View the logs of a particular build or deployment by version if greater than zero")

	return cmd
}

// Complete calls the upstream Complete for the logs command and then resolves the
// resource a user requested to view its logs and creates the appropriate logOptions
// object for it.
func (o *OpenShiftLogsOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if err := o.KubeLogOptions.Complete(f.Factory, out, cmd, args); err != nil {
		return err
	}
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	podLogOptions := o.KubeLogOptions.Options.(*kapi.PodLogOptions)

	mapper, typer := f.Object(false)
	infos, err := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
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

	version := kcmdutil.GetFlagInt64(cmd, "version")
	_, resource := meta.KindToResource(infos[0].Mapping.GroupVersionKind)

	// TODO: podLogOptions should be included in our own logOptions objects.
	switch resource.GroupResource() {
	case buildapi.Resource("build"), buildapi.Resource("buildconfig"):
		bopts := &buildapi.BuildLogOptions{
			Follow:       podLogOptions.Follow,
			Previous:     podLogOptions.Previous,
			SinceSeconds: podLogOptions.SinceSeconds,
			SinceTime:    podLogOptions.SinceTime,
			Timestamps:   podLogOptions.Timestamps,
			TailLines:    podLogOptions.TailLines,
			LimitBytes:   podLogOptions.LimitBytes,
		}
		if version != 0 {
			bopts.Version = &version
		}
		o.Options = bopts
	case deployapi.Resource("deploymentconfig"):
		dopts := &deployapi.DeploymentLogOptions{
			Follow:       podLogOptions.Follow,
			Previous:     podLogOptions.Previous,
			SinceSeconds: podLogOptions.SinceSeconds,
			SinceTime:    podLogOptions.SinceTime,
			Timestamps:   podLogOptions.Timestamps,
			TailLines:    podLogOptions.TailLines,
			LimitBytes:   podLogOptions.LimitBytes,
		}
		if version != 0 {
			dopts.Version = &version
		}
		o.Options = dopts
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
	switch t := o.Options.(type) {
	case *buildapi.BuildLogOptions:
		if t.Previous && t.Version != nil {
			return errors.New("cannot use both --previous and --version")
		}
	case *deployapi.DeploymentLogOptions:
		if t.Previous && t.Version != nil {
			return errors.New("cannot use both --previous and --version")
		}
	default:
		return errors.New("invalid log options object provided")
	}
	return nil
}

// RunLog will run the upstream logs command and may use an OpenShift
// logOptions object.
func (o OpenShiftLogsOptions) RunLog() error {
	if o.Options != nil {
		// Use our own options object.
		o.KubeLogOptions.Options = o.Options
	}
	_, err := o.KubeLogOptions.RunLogs()
	return err
}
