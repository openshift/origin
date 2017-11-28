package rollout

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kapps "k8s.io/kubernetes/pkg/apis/apps"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"
	kextensions "k8s.io/kubernetes/pkg/apis/extensions"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller/deployment/util"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclientinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	deployutil "github.com/openshift/origin/pkg/apps/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	triggerapi "github.com/openshift/origin/pkg/image/apis/image/v1/trigger"
	imageclientinternal "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	"github.com/openshift/origin/pkg/image/trigger/annotations"
	triggerresolve "github.com/openshift/origin/pkg/image/trigger/resolve"
)

var (
	rolloutLatestLong = templates.LongDesc(`
		Start a new rollout for a deployment config with the latest state from its triggers

		This command is appropriate for running manual rollouts. If you want full control over
		running new rollouts, use "oc set triggers --manual" to disable all triggers in your
		deployment config and then whenever you want to run a new deployment process, use this
		command in order to pick up the latest images found in the cluster that are pointed by
		your image change triggers.`)

	rolloutLatestExample = templates.Examples(`
		# Start a new rollout based on the latest images defined in the image change triggers.
  	%[1]s rollout latest dc/nginx`)
)

// RolloutLatestOptions holds all the options for the `rollout latest` command.
type RolloutLatestOptions struct {
	mapper meta.RESTMapper
	typer  runtime.ObjectTyper
	infos  []*resource.Info

	DryRun bool
	out    io.Writer
	output string
	again  bool

	appsClient      appsclientinternal.DeploymentConfigsGetter
	imageClient     imageclientinternal.ImageStreamTagsGetter
	kc              kclientset.Interface
	baseCommandName string
}

// NewCmdRolloutLatest implements the oc rollout latest subcommand.
func NewCmdRolloutLatest(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &RolloutLatestOptions{
		baseCommandName: fullName,
	}

	cmd := &cobra.Command{
		Use:     "latest RESOURCE/NAME",
		Short:   "Start a new rollout for a resource with the latest state from its triggers",
		Long:    rolloutLatestLong,
		Example: fmt.Sprintf(rolloutLatestExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := opts.Complete(f, cmd, args, out)
			kcmdutil.CheckErr(err)

			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			err = opts.RunRolloutLatest()
			kcmdutil.CheckErr(err)
		},
		ValidArgs: []string{"deploymentconfig"},
	}

	kcmdutil.AddPrinterFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)
	cmd.Flags().Bool("again", false, "If true, deploy the current pod template without updating state from triggers")

	return cmd
}

func (o *RolloutLatestOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	if len(args) != 1 {
		return errors.New("one deployment config name is needed as argument")
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetFlagBool(cmd, "dry-run")

	o.kc, err = f.ClientSet()
	if err != nil {
		return err
	}
	appsClient, err := f.OpenshiftInternalAppsClient()
	if err != nil {
		return err
	}
	o.appsClient = appsClient.Apps()

	imageClient, err := f.OpenshiftInternalImageClient()
	if err != nil {
		return err
	}
	o.imageClient = imageClient.Image()

	o.mapper, o.typer = f.Object()
	o.infos, err = f.NewBuilder(true).
		ContinueOnError().
		NamespaceParam(namespace).
		ResourceNames("deploymentconfigs", args[0]).
		SingleResourceType().
		Do().Infos()
	if err != nil {
		return err
	}

	o.out = out
	o.output = kcmdutil.GetFlagString(cmd, "output")
	o.again = kcmdutil.GetFlagBool(cmd, "again")

	return nil
}

func (o RolloutLatestOptions) Validate() error {
	if len(o.infos) != 1 {
		return errors.New("a resource name is required")
	}
	return nil
}

// rolloutFromAnnotation will rollout any type that support annotation trigger
// and the trigger is currently paused.
func (o RolloutLatestOptions) rolloutFromAnnotation(obj runtime.Object, updateFn func(runtime.Object) (runtime.Object, error)) (runtime.Object, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	out, ok := accessor.GetAnnotations()[triggerapi.TriggerAnnotationKey]
	if !ok {
		return nil, fmt.Errorf("%s does not have any triggers defined", accessor.GetName())
	}
	triggers := []triggerapi.ObjectFieldTrigger{}
	if err := json.Unmarshal([]byte(out), &triggers); err != nil {
		return nil, err
	}
	var (
		errors                []error
		hasPausedImageTrigger bool
		hasUpdatedImages      bool
	)
	for _, trigger := range triggers {
		if !trigger.Paused {
			continue
		}
		container, remainder, err := annotations.ContainerForObjectFieldPath(obj, trigger.FieldPath)
		if err != nil || remainder != "image" {
			continue
		}
		hasPausedImageTrigger = true
		spec, err := triggerresolve.LatestTriggerImagePullSpec(o.imageClient, accessor.GetNamespace(), trigger.From)
		if err != nil {
			errors = append(errors, fmt.Errorf("unable to resolve image %s for container %q: %v", trigger.From.Name, container.GetName(), err))
			continue
		}
		if container.GetImage() != spec.String() {
			container.SetImage(spec.String())
			hasUpdatedImages = true
		}
	}
	if !hasPausedImageTrigger {
		return nil, fmt.Errorf("%q has no paused triggers", accessor.GetName())
	}
	if !hasUpdatedImages {
		return nil, fmt.Errorf("%q already runs the latest images", accessor.GetName())
	}
	if len(errors) > 0 {
		return nil, fmt.Errorf("%q: %v", accessor.GetName(), utilerrors.NewAggregate(errors))
	}
	return updateFn(obj)
}

// rolloutDeploymentConfig rollouts the latest version for deployment config.
func (o RolloutLatestOptions) rolloutDeploymentConfig(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	// TODO: Consider allowing one-off deployments for paused configs
	// See https://github.com/openshift/origin/issues/9903
	if config.Spec.Paused {
		return nil, fmt.Errorf("cannot deploy a paused deployment config")
	}

	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := o.kc.Core().ReplicationControllers(config.Namespace).Get(deploymentName, metav1.GetOptions{})
	switch {
	case err == nil:
		// Reject attempts to start a concurrent deployment.
		if !deployutil.IsTerminatedDeployment(deployment) {
			status := deployutil.DeploymentStatusFor(deployment)
			return nil, fmt.Errorf("#%d is already in progress (%s)", config.Status.LatestVersion, status)
		}
	case !kerrors.IsNotFound(err):
		return nil, err
	}

	dc := config
	if !o.DryRun {
		request := &deployapi.DeploymentRequest{
			Name:   config.Name,
			Latest: !o.again,
			Force:  true,
		}

		dc, err = o.appsClient.DeploymentConfigs(config.Namespace).Instantiate(config.Name, request)

		// Pre 1.4 servers don't support the instantiate endpoint. Fallback to incrementing
		// latestVersion on them.
		if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
			config.Status.LatestVersion++
			dc, err = o.appsClient.DeploymentConfigs(config.Namespace).Update(config)
		}

		if err != nil {
			return nil, err
		}
	}

	return dc, nil
}

// RunRolloutLatest runs the latest rollouts.
func (o RolloutLatestOptions) RunRolloutLatest() error {
	var revision string
	info := o.infos[0]
	switch obj := info.Object.(type) {
	case *deployapi.DeploymentConfig:
		dc, err := o.rolloutDeploymentConfig(obj)
		if err != nil {
			return err
		}
		revision = fmt.Sprintf("%d", dc.Status.LatestVersion)
		info.Refresh(dc, true)
	case *kextensions.Deployment:
		updatedObj, err := o.rolloutFromAnnotation(obj, func(in runtime.Object) (runtime.Object, error) {
			return o.kc.Extensions().Deployments(obj.Namespace).Update(in.(*kextensions.Deployment))
		})
		if err != nil {
			return fmt.Errorf("%s %v", info.Mapping.Resource, err)
		}
		currentRevision, _ := util.Revision(updatedObj)
		revision = fmt.Sprintf("%d", currentRevision)
		info.Refresh(updatedObj, true)
	case *kextensions.DaemonSet:
		updatedObj, err := o.rolloutFromAnnotation(obj, func(in runtime.Object) (runtime.Object, error) {
			return o.kc.Extensions().DaemonSets(obj.Namespace).Update(in.(*kextensions.DaemonSet))
		})
		if err != nil {
			return fmt.Errorf("%s %v", info.Mapping.Resource, err)
		}
		// TODO: Does DaemonSets have revision?
		info.Refresh(updatedObj, true)
	case *kapps.StatefulSet:
		updatedObj, err := o.rolloutFromAnnotation(obj, func(in runtime.Object) (runtime.Object, error) {
			return o.kc.Apps().StatefulSets(obj.Namespace).Update(in.(*kapps.StatefulSet))
		})
		if err != nil {
			return fmt.Errorf("%s %v", info.Mapping.Resource, err)
		}
		s := updatedObj.(*kapps.StatefulSet)
		revision = s.Status.CurrentRevision
		info.Refresh(updatedObj, true)
	case *kbatch.CronJob:
		updatedObj, err := o.rolloutFromAnnotation(obj, func(in runtime.Object) (runtime.Object, error) {
			return o.kc.Batch().CronJobs(obj.Namespace).Update(in.(*kbatch.CronJob))
		})
		if err != nil {
			return fmt.Errorf("%s %v", info.Mapping.Resource, err)
		}
		info.Refresh(updatedObj, true)
	default:
		return fmt.Errorf("manual rollouts are not supported for %s", info.Mapping.Resource)
	}

	if o.output == "revision" && len(revision) > 0 {
		fmt.Fprintf(o.out, revision)
		return nil
	}
	kcmdutil.PrintSuccess(o.mapper, o.output == "name", o.out, info.Mapping.Resource, info.Name, o.DryRun, "rolled out")
	return nil
}
