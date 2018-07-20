package rollout

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsinternalutil "github.com/openshift/origin/pkg/apps/controller/util"
	"github.com/openshift/origin/pkg/oc/cli/set"
)

type CancelOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Builder           func() *resource.Builder
	Namespace         string
	NamespaceExplicit bool
	Mapper            meta.RESTMapper
	Encoder           runtime.Encoder
	Resources         []string
	Clientset         kclientset.Interface

	Printer func(string) (printers.ResourcePrinter, error)

	resource.FilenameOptions
	genericclioptions.IOStreams
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

func NewRolloutCancelOptions(streams genericclioptions.IOStreams) *CancelOptions {
	return &CancelOptions{
		IOStreams:  streams,
		PrintFlags: genericclioptions.NewPrintFlags("already cancelled"),
	}
}

func NewCmdRolloutCancel(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRolloutCancelOptions(streams)

	cmd := &cobra.Command{
		Use:     "cancel (TYPE NAME | TYPE/NAME) [flags]",
		Long:    rolloutCancelLong,
		Example: fmt.Sprintf(rolloutCancelExample, fullName),
		Short:   "cancel the in-progress deployment",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
		ValidArgs: []string{"deploymentconfig"},
	}

	o.PrintFlags.AddFlags(cmd)

	usage := "Filename, directory, or URL to a file identifying the resource to get from a server."
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	return cmd
}

func (o *CancelOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) == 0 && len(o.FilenameOptions.Filenames) == 0 {
		return kcmdutil.UsageErrorf(cmd, "a resource or filename must be specified")
	}

	var err error
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}
	o.Encoder = kcmdutil.InternalVersionJSONEncoder()

	o.Namespace, o.NamespaceExplicit, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Clientset, err = f.ClientSet()
	if err != nil {
		return err
	}

	o.Printer = func(successMsg string) (printers.ResourcePrinter, error) {
		o.PrintFlags.Complete(successMsg)
		return o.PrintFlags.ToPrinter()
	}

	o.Builder = f.NewBuilder
	o.Resources = args
	return nil
}

func (o CancelOptions) Run() error {
	r := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.NamespaceExplicit, &o.FilenameOptions).
		ResourceTypeOrNameArgs(true, o.Resources...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	allErrs := []error{}
	for _, info := range infos {
		config, ok := info.Object.(*appsv1.DeploymentConfig)
		if !ok {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, fmt.Errorf("expected deployment configuration, got %s", info.Mapping.Resource.Resource)))
			continue
		}
		if config.Spec.Paused {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, fmt.Errorf("unable to cancel paused deployment %s/%s", config.Namespace, config.Name)))
		}

		mapping, err := o.Mapper.RESTMapping(kapi.Kind("ReplicationController"))
		if err != nil {
			return err
		}

		mutateFn := func(rc *kapi.ReplicationController) bool {
			printer, err := o.Printer("already cancelled")
			if err != nil {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, err))
				return false
			}

			if appsinternalutil.IsDeploymentCancelled(rc) {
				printer.PrintObj(info.Object, o.Out)
				return false
			}

			patches := set.CalculatePatchesExternal([]*resource.Info{{Object: rc, Mapping: mapping}}, func(info *resource.Info) (bool, error) {
				rc.Annotations[appsapi.DeploymentCancelledAnnotation] = appsapi.DeploymentCancelledAnnotationValue
				rc.Annotations[appsapi.DeploymentStatusReasonAnnotation] = appsapi.DeploymentCancelledByUser
				return true, nil
			})

			allPatchesEmpty := true
			for _, patch := range patches {
				if len(patch.Patch) > 0 {
					allPatchesEmpty = false
					break
				}
			}
			if allPatchesEmpty {
				printer.PrintObj(info.Object, o.Out)
				return false
			}

			if _, err := o.Clientset.Core().ReplicationControllers(rc.Namespace).Patch(rc.Name, types.StrategicMergePatchType, patches[0].Patch); err != nil {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, err))
				return false
			}

			printer, err = o.Printer("cancelling")
			if err != nil {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("cancelling", info.Source, err))
				return false
			}
			printer.PrintObj(info.Object, o.Out)
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
			if appsinternalutil.IsDeploymentCancelled(latest) && !appsinternalutil.IsTerminatedDeployment(latest) {
				maybeCancelling = " (cancelling)"
			}
			timeAt := strings.ToLower(units.HumanDuration(time.Now().Sub(latest.CreationTimestamp.Time)))
			fmt.Fprintf(o.Out, "No rollout is in progress (latest rollout #%d %s%s %s ago)\n",
				appsinternalutil.DeploymentVersionFor(latest),
				strings.ToLower(string(appsinternalutil.DeploymentStatusFor(latest))),
				maybeCancelling,
				timeAt)
		}

	}
	return utilerrors.NewAggregate(allErrs)
}

func (o CancelOptions) forEachControllerInConfig(namespace, name string, mutateFunc func(*kapi.ReplicationController) bool) ([]*kapi.ReplicationController, bool, error) {
	deploymentList, err := o.Clientset.Core().ReplicationControllers(namespace).List(metav1.ListOptions{LabelSelector: appsinternalutil.ConfigSelector(name).String()})
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
	sort.Sort(appsinternalutil.ByLatestVersionDesc(deployments))
	allErrs := []error{}
	cancelled := false

	for _, deployment := range deployments {
		status := appsinternalutil.DeploymentStatusFor(deployment)
		switch status {
		case appsapi.DeploymentStatusNew,
			appsapi.DeploymentStatusPending,
			appsapi.DeploymentStatusRunning:
			cancelled = mutateFunc(deployment)
		}
	}

	return deployments, cancelled, utilerrors.NewAggregate(allErrs)
}
