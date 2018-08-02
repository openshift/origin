package logs

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
	buildv1 "github.com/openshift/api/build/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildutil "github.com/openshift/origin/pkg/build/util"
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

// LogsOptions holds all the necessary options for running oc logs.
type LogsOptions struct {
	// Options should hold our own *LogOptions objects.
	Options runtime.Object
	// KubeLogOptions contains all the necessary options for
	// running the upstream logs command.
	KubeLogOptions *kcmd.LogsOptions
	// Client enables access to the Build object when processing
	// build logs for Jenkins Pipeline Strategy builds
	Client buildv1client.BuildV1Interface
	// Namespace is a required parameter when accessing the Build object when processing
	// build logs for Jenkins Pipeline Strategy builds
	Namespace string

	Builder   func() *resource.Builder
	Resources []string

	Version int64

	genericclioptions.IOStreams
}

func NewLogsOptions(streams genericclioptions.IOStreams) *LogsOptions {
	return &LogsOptions{
		KubeLogOptions: kcmd.NewLogsOptions(streams, false),
		IOStreams:      streams,
	}
}

// NewCmdLogs creates a new logs command that supports OpenShift resources.
func NewCmdLogs(name, baseName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewLogsOptions(streams)
	cmd := kcmd.NewCmdLogs(f, streams)
	cmd.Short = "Print the logs for a resource"
	cmd.Long = logsLong
	cmd.Example = fmt.Sprintf(logsExample, baseName, name)
	cmd.SuggestFor = []string{"builds", "deployments"}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		kcmdutil.CheckErr(o.Complete(f, cmd, args))
		kcmdutil.CheckErr(o.Validate(args))
		kcmdutil.CheckErr(o.RunLog())
	}

	cmd.Flags().Int64Var(&o.Version, "version", o.Version, "View the logs of a particular build or deployment by version if greater than zero")

	return cmd
}

func isPipelineBuild(obj runtime.Object) (bool, *buildv1.BuildConfig, bool, *buildv1.Build, bool) {
	bc, isBC := obj.(*buildv1.BuildConfig)
	build, isBld := obj.(*buildv1.Build)
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
func (o *LogsOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	// manually bind all flag values from the upstream command
	// TODO: once the upstream command supports binding flags
	// by outside callers, this will no longer be needed.
	o.KubeLogOptions.AllContainers = kcmdutil.GetFlagBool(cmd, "all-containers")
	o.KubeLogOptions.Container = kcmdutil.GetFlagString(cmd, "container")
	o.KubeLogOptions.Selector = kcmdutil.GetFlagString(cmd, "selector")
	o.KubeLogOptions.Follow = kcmdutil.GetFlagBool(cmd, "follow")
	o.KubeLogOptions.Previous = kcmdutil.GetFlagBool(cmd, "previous")
	o.KubeLogOptions.Timestamps = kcmdutil.GetFlagBool(cmd, "timestamps")
	o.KubeLogOptions.SinceTime = kcmdutil.GetFlagString(cmd, "since-time")
	o.KubeLogOptions.LimitBytes = kcmdutil.GetFlagInt64(cmd, "limit-bytes")
	o.KubeLogOptions.Tail = kcmdutil.GetFlagInt64(cmd, "tail")
	o.KubeLogOptions.SinceSeconds = kcmdutil.GetFlagDuration(cmd, "since")
	o.KubeLogOptions.ContainerNameSpecified = cmd.Flag("container").Changed

	if err := o.KubeLogOptions.Complete(f, cmd, args); err != nil {
		return err
	}

	var err error
	o.KubeLogOptions.GetPodTimeout, err = kcmdutil.GetPodRunningTimeoutFlag(cmd)
	if err != nil {
		return err
	}

	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = buildv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.Builder = f.NewBuilder
	o.Resources = args

	return nil
}

// Validate runs the upstream validation for the logs command and then it
// will validate any OpenShift-specific log options.
func (o *LogsOptions) Validate(args []string) error {
	if err := o.KubeLogOptions.Validate(); err != nil {
		return err
	}
	if o.Options == nil {
		return nil
	}
	switch t := o.Options.(type) {
	case *buildv1.BuildLogOptions:
		if t.Previous && t.Version != nil {
			return errors.New("cannot use both --previous and --version")
		}
	case *appsv1.DeploymentLogOptions:
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
func (o *LogsOptions) RunLog() error {
	podLogOptions := o.KubeLogOptions.Options.(*corev1.PodLogOptions)
	infos, err := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		ResourceNames("pods", o.Resources...).
		SingleResourceType().RequireObject(false).
		Do().Infos()
	if err != nil {
		return err
	}
	if len(infos) != 1 {
		return errors.New("expected a resource")
	}

	// TODO: podLogOptions should be included in our own logOptions objects.
	switch gr := infos[0].Mapping.Resource.GroupResource(); gr {
	case buildv1.Resource("builds"),
		buildv1.Resource("buildconfigs"):
		bopts := &buildv1.BuildLogOptions{
			Follow:       podLogOptions.Follow,
			Previous:     podLogOptions.Previous,
			SinceSeconds: podLogOptions.SinceSeconds,
			SinceTime:    podLogOptions.SinceTime,
			Timestamps:   podLogOptions.Timestamps,
			TailLines:    podLogOptions.TailLines,
			LimitBytes:   podLogOptions.LimitBytes,
		}
		if o.Version != 0 {
			bopts.Version = &o.Version
		}
		o.Options = bopts

	case appsv1.Resource("deploymentconfigs"):
		dopts := &appsv1.DeploymentLogOptions{
			Container:    podLogOptions.Container,
			Follow:       podLogOptions.Follow,
			Previous:     podLogOptions.Previous,
			SinceSeconds: podLogOptions.SinceSeconds,
			SinceTime:    podLogOptions.SinceTime,
			Timestamps:   podLogOptions.Timestamps,
			TailLines:    podLogOptions.TailLines,
			LimitBytes:   podLogOptions.LimitBytes,
		}
		if o.Version != 0 {
			dopts.Version = &o.Version
		}
		o.Options = dopts
	default:
		o.Options = nil
	}

	return o.runLogPipeline()
}

func (o *LogsOptions) runLogPipeline() error {
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
			return fmt.Errorf("the build %s for build config %s was not found", buildName, bc.Name)
		}
		fallthrough
	case isBld:
		urlString, _ := build.Annotations[buildapi.BuildJenkinsBlueOceanLogURLAnnotation]
		if len(urlString) == 0 {
			return fmt.Errorf("the pipeline strategy build %s does not yet contain the log URL; wait a few moments, then try again", build.Name)
		}
		fmt.Fprintf(o.Out, "info: logs available at %s\n", urlString)
	default:
		return fmt.Errorf("a pipeline strategy build log operation peformed against invalid object %#v", o.KubeLogOptions.Object)
	}

	return nil
}
