package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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
	// Client enables access to the Build object when processing
	// build logs for Jenkins Pipeline Strategy builds
	Client buildclient.BuildsGetter
	// Namespace is a required parameter when accessing the Build object when processing
	// build logs for Jenkins Pipeline Strategy builds
	Namespace string
}

// NewCmdLogs creates a new logs command that supports OpenShift resources.
func NewCmdLogs(name, baseName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := OpenShiftLogsOptions{
		KubeLogOptions: &kcmd.LogsOptions{},
	}

	cmd := kcmd.NewCmdLogs(f, out)
	cmd.Short = "Print the logs for a resource"
	cmd.Long = logsLong
	cmd.Example = fmt.Sprintf(logsExample, baseName, name)
	cmd.SuggestFor = []string{"builds", "deployments"}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		kcmdutil.CheckErr(o.Complete(f, cmd, args, out))

		if err := o.Validate(); err != nil {
			kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
		}

		kcmdutil.CheckErr(o.RunLog())
	}

	cmd.Flags().Int64("version", 0, "View the logs of a particular build or deployment by version if greater than zero")

	return cmd
}

func isPipelineBuild(obj runtime.Object) (bool, *buildapi.BuildConfig, bool, *buildapi.Build, bool) {
	bc, isBC := obj.(*buildapi.BuildConfig)
	build, isBld := obj.(*buildapi.Build)
	isPipeline := false
	switch {
	case isBC:
		isPipeline = bc.Spec.CommonSpec.Strategy.JenkinsPipelineStrategy != nil
	case isBld:
		isPipeline = build.Spec.CommonSpec.Strategy.JenkinsPipelineStrategy != nil
	}
	return isPipeline, bc, isBC, build, isBld
}

// Complete calls the upstream Complete for the logs command and then resolves the
// resource a user requested to view its logs and creates the appropriate logOptions
// object for it.
func (o *OpenShiftLogsOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if err := o.KubeLogOptions.Complete(f, out, cmd, args); err != nil {
		return err
	}
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace

	podLogOptions := o.KubeLogOptions.Options.(*kapi.PodLogOptions)

	infos, err := f.NewBuilder().
		Internal().
		NamespaceParam(o.Namespace).DefaultNamespace().
		ResourceNames("pods", args...).
		SingleResourceType().RequireObject(false).
		Do().Infos()
	if err != nil {
		return err
	}
	if len(infos) != 1 {
		return errors.New("expected a resource")
	}

	client, err := f.OpenshiftInternalBuildClient()
	if err != nil {
		return err
	}
	o.Client = client.Build()

	version := kcmdutil.GetFlagInt64(cmd, "version")
	_, resource := meta.UnsafeGuessKindToResource(infos[0].Mapping.GroupVersionKind)

	gr := resource.GroupResource()
	// TODO: podLogOptions should be included in our own logOptions objects.
	switch {
	case buildapi.IsResourceOrLegacy("build", gr), buildapi.IsResourceOrLegacy("buildconfig", gr):
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

	case appsapi.IsResourceOrLegacy("deploymentconfig", gr):
		dopts := &appsapi.DeploymentLogOptions{
			Container:    podLogOptions.Container,
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
	case *appsapi.DeploymentLogOptions:
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
	isPipeline, bc, isBC, build, isBld := isPipelineBuild(o.KubeLogOptions.Object)
	if !isPipeline {
		return o.KubeLogOptions.RunLogs()
	}

	switch {
	case isBC:
		buildName := buildutil.BuildNameForConfigVersion(bc.ObjectMeta.Name, int(bc.Status.LastVersion))
		build, _ = o.Client.Builds(o.Namespace).Get(buildName, metav1.GetOptions{})
		if build == nil {
			return errors.New(fmt.Sprintf("The build %s for build config %s was not found", buildName, bc.ObjectMeta.Name))
		}
		fallthrough
	case isBld:
		urlString, _ := build.Annotations[buildapi.BuildJenkinsBlueOceanLogURLAnnotation]
		if len(urlString) == 0 {
			return errors.New(fmt.Sprintf("The pipeline strategy build %s does not yet contain the log URL; wait a few moments, then try again", build.ObjectMeta.Name))
		}
		o.KubeLogOptions.Out.Write([]byte(fmt.Sprintf("info: Logs available at %s\n", urlString)))
	default:
		return errors.New(fmt.Sprintf("A pipeline strategy build log operation peformed against invalid object %#v", o.KubeLogOptions.Object))
	}

	return nil
}
