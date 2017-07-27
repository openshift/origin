package rollout

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	units "github.com/docker/go-units"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	"github.com/spf13/cobra"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/set"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

type CancelOptions struct {
	Mapper  meta.RESTMapper
	Typer   runtime.ObjectTyper
	Encoder runtime.Encoder
	Infos   []*resource.Info

	Out             io.Writer
	FilenameOptions resource.FilenameOptions

	Clientset kclientset.Interface
}

var (
	rolloutCancelLong = templates.LongDesc(`
Cancel the in-progress deployment

Running this command will cause the current in-progress deployment to be
cancelled, but keep in mind that this is a best-effort operation and may take
some time to complete. It’s possible the deployment will partially or totally
complete before the cancellation is effective. In such a case an appropriate
event will be emitted.`)

	rolloutCancelExample = templates.Examples(`
	# Cancel the in-progress deployment based on 'nginx'
  %[1]s rollout cancel dc/nginx`)
)

func NewCmdRolloutCancel(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &CancelOptions{}
	cmd := &cobra.Command{
		Use:     "cancel (TYPE NAME | TYPE/NAME) [flags]",
		Long:    rolloutCancelLong,
		Example: fmt.Sprintf(rolloutCancelExample, fullName),
		Short:   "cancel the in-progress deployment",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, out, args))
			kcmdutil.CheckErr(opts.Run())
		},
	}
	usage := "Filename, directory, or URL to a file identifying the resource to get from a server."
	kcmdutil.AddFilenameOptionFlags(cmd, &opts.FilenameOptions, usage)
	return cmd
}

func (o *CancelOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer, args []string) error {
	if len(args) == 0 && len(o.FilenameOptions.Filenames) == 0 {
		return kcmdutil.UsageError(cmd, cmd.Use)
	}

	o.Mapper, o.Typer = f.Object()
	o.Encoder = f.JSONEncoder()
	o.Out = out

	cmdNamespace, enforceNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Clientset, err = f.ClientSet()
	if err != nil {
		return err
	}

	r := resource.NewBuilder(o.Mapper, f.CategoryExpander(), o.Typer, resource.ClientMapperFunc(f.ClientForMapping), f.Decoder(true)).
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(enforceNamespace, &o.FilenameOptions).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}

	o.Infos, err = r.Infos()
	return err
}

func (o CancelOptions) Run() error {
	allErrs := []error{}
	for _, info := range o.Infos {
		config, ok := info.Object.(*deployapi.DeploymentConfig)
		if !ok {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, fmt.Errorf("expected deployment configuration, got %T", info.Object)))
		}
		if config.Spec.Paused {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, fmt.Errorf("unable to cancel paused deployment %s/%s", config.Namespace, config.Name)))
		}

		mapping, err := o.Mapper.RESTMapping(kapi.Kind("ReplicationController"))
		if err != nil {
			return err
		}

		mutateFn := func(rc *kapi.ReplicationController) bool {
			if deployutil.IsDeploymentCancelled(rc) {
				kcmdutil.PrintSuccess(o.Mapper, false, o.Out, info.Mapping.Resource, info.Name, false, "already cancelled")
				return false
			}

			patches := set.CalculatePatches([]*resource.Info{{Object: rc, Mapping: mapping}}, o.Encoder, func(*resource.Info) ([]byte, error) {
				rc.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
				rc.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledByUser
				return runtime.Encode(o.Encoder, rc)
			})

			if len(patches) == 0 {
				kcmdutil.PrintSuccess(o.Mapper, false, o.Out, info.Mapping.Resource, info.Name, false, "already cancelled")
				return false
			}

			_, err := o.Clientset.Core().ReplicationControllers(rc.Namespace).Patch(rc.Name, types.StrategicMergePatchType, patches[0].Patch)
			if err != nil {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, err))
				return false
			}
			kcmdutil.PrintSuccess(o.Mapper, false, o.Out, info.Mapping.Resource, info.Name, false, "cancelling")
			return true
		}

		deployments, cancelled, err := o.forEachControllerInConfig(config.Namespace, config.Name, mutateFn)
		if err != nil {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, err))
			continue
		}

		if !cancelled {
			latest := deployments[0]
			maybeCancelling := ""
			if deployutil.IsDeploymentCancelled(latest) && !deployutil.IsTerminatedDeployment(latest) {
				maybeCancelling = " (cancelling)"
			}
			timeAt := strings.ToLower(units.HumanDuration(time.Now().Sub(latest.CreationTimestamp.Time)))
			fmt.Fprintf(o.Out, "No rollout is in progress (latest rollout #%d %s%s %s ago)\n",
				deployutil.DeploymentVersionFor(latest),
				strings.ToLower(string(deployutil.DeploymentStatusFor(latest))),
				maybeCancelling,
				timeAt)
		}

	}
	return utilerrors.NewAggregate(allErrs)
}

func (o CancelOptions) forEachControllerInConfig(namespace, name string, mutateFunc func(*kapi.ReplicationController) bool) ([]*kapi.ReplicationController, bool, error) {
	deploymentList, err := o.Clientset.Core().ReplicationControllers(namespace).List(metav1.ListOptions{LabelSelector: deployutil.ConfigSelector(name).String()})
	if err != nil {
		return nil, false, err
	}
	if len(deploymentList.Items) == 0 {
		return nil, false, fmt.Errorf("there have been no replication controllers for %s/%s\n", namespace, name)
	}
	deployments := make([]*kapi.ReplicationController, 0, len(deploymentList.Items))
	for i := range deploymentList.Items {
		deployments = append(deployments, &deploymentList.Items[i])
	}
	sort.Sort(deployutil.ByLatestVersionDesc(deployments))
	allErrs := []error{}
	cancelled := false

	for _, deployment := range deployments {
		status := deployutil.DeploymentStatusFor(deployment)
		switch status {
		case deployapi.DeploymentStatusNew,
			deployapi.DeploymentStatusPending,
			deployapi.DeploymentStatusRunning:
			cancelled = mutateFunc(deployment)
		}
	}

	return deployments, cancelled, utilerrors.NewAggregate(allErrs)
}
