package rollout

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kexternalclientset "k8s.io/client-go/kubernetes"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	appsv1 "github.com/openshift/api/apps/v1"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	"github.com/openshift/origin/pkg/oc/cli/set"
)

type RetryOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Resources         []string
	Builder           func() *resource.Builder
	Mapper            meta.RESTMapper
	Encoder           runtime.Encoder
	Clientset         kexternalclientset.Interface
	Namespace         string
	ExplicitNamespace bool

	ToPrinter func(string) (printers.ResourcePrinter, error)

	resource.FilenameOptions
	genericclioptions.IOStreams
}

var (
	rolloutRetryLong = templates.LongDesc(`
		If a rollout fails, you may opt to retry it (if the error was transient). Some rollouts may
		never successfully complete - in which case you can use the rollout latest to force a redeployment.
		If a deployment config has completed rolling out successfully at least once in the past, it would be
		automatically rolled back in the event of a new failed rollout. Note that you would still need
		to update the erroneous deployment config in order to have its template persisted across your
		application.
`)

	rolloutRetryExample = templates.Examples(`
	  # Retry the latest failed deployment based on 'frontend'
	  # The deployer pod and any hook pods are deleted for the latest failed deployment
	  %[1]s rollout retry dc/frontend
`)
)

func NewRolloutRetryOptions(streams genericclioptions.IOStreams) *RetryOptions {
	return &RetryOptions{
		PrintFlags: genericclioptions.NewPrintFlags("already retried").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

func NewCmdRolloutRetry(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRolloutRetryOptions(streams)
	cmd := &cobra.Command{
		Use:     "retry (TYPE NAME | TYPE/NAME) [flags]",
		Long:    rolloutRetryLong,
		Example: fmt.Sprintf(rolloutRetryExample, fullName),
		Short:   "Retry the latest failed rollout",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)

	usage := "Filename, directory, or URL to a file identifying the resource to get from a server."
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	return cmd
}

func (o *RetryOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	if len(args) == 0 && len(o.FilenameOptions.Filenames) == 0 {
		return kcmdutil.UsageErrorf(cmd, cmd.Use)
	}

	o.Resources = args

	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}
	o.Encoder = kcmdutil.InternalVersionJSONEncoder()

	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.Clientset, err = kexternalclientset.NewForConfig(config)
	if err != nil {
		return err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	o.Builder = f.NewBuilder

	return err
}

func (o RetryOptions) Run() error {
	allErrs := []error{}
	mapping, err := o.Mapper.RESTMapping(kapi.Kind("ReplicationController"))
	if err != nil {
		return err
	}

	r := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
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

	for _, info := range infos {
		config, ok := info.Object.(*appsv1.DeploymentConfig)
		if !ok {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("expected deployment configuration, got %T", info.Object)))
			continue
		}
		if config.Spec.Paused {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("unable to retry paused deployment config %q", config.Name)))
			continue
		}
		if config.Status.LatestVersion == 0 {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("no rollouts found for %q", config.Name)))
			continue
		}

		latestDeploymentName := appsutil.LatestDeploymentNameForConfig(config)
		rc, err := o.Clientset.Core().ReplicationControllers(config.Namespace).Get(latestDeploymentName, metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("unable to find the latest rollout (#%d).\nYou can start a new rollout with 'oc rollout latest dc/%s'.", config.Status.LatestVersion, config.Name)))
				continue
			}
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("unable to fetch replication controller %q", config.Name)))
			continue
		}

		if !appsutil.IsFailedDeployment(rc) {
			message := fmt.Sprintf("rollout #%d is %s; only failed deployments can be retried.\n", config.Status.LatestVersion,
				strings.ToLower(appsutil.AnnotationFor(rc, appsv1.DeploymentStatusAnnotation)))
			if appsutil.IsCompleteDeployment(rc) {
				message += fmt.Sprintf("You can start a new deployment with 'oc rollout latest dc/%s'.", config.Name)
			} else {
				message += fmt.Sprintf("Optionally, you can cancel this deployment with 'oc rollout cancel dc/%s'.", config.Name)
			}
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, errors.New(message)))
			continue
		}

		// Delete the deployer pod as well as the deployment hooks pods, if any
		pods, err := o.Clientset.Core().Pods(config.Namespace).List(metav1.ListOptions{LabelSelector: appsutil.DeployerPodSelector(
			latestDeploymentName).String()})
		if err != nil {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("failed to list deployer/hook pods for deployment #%d: %v", config.Status.LatestVersion, err)))
			continue
		}
		hasError := false
		for _, pod := range pods.Items {
			err := o.Clientset.Core().Pods(pod.Namespace).Delete(pod.Name, metav1.NewDeleteOptions(0))
			if err != nil {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("failed to delete deployer/hook pod %s for deployment #%d: %v", pod.Name, config.Status.LatestVersion, err)))
				hasError = true
			}
		}
		if hasError {
			continue
		}

		patches := set.CalculatePatchesExternal([]*resource.Info{{Object: rc, Mapping: mapping}}, func(info *resource.Info) (bool, error) {
			rc.Annotations[appsv1.DeploymentStatusAnnotation] = string(appsv1.DeploymentStatusNew)
			delete(rc.Annotations, appsv1.DeploymentStatusReasonAnnotation)
			delete(rc.Annotations, appsv1.DeploymentCancelledAnnotation)
			return true, nil
		})

		if len(patches) == 0 {
			printer, err := o.ToPrinter("already retried")
			if err != nil {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, err))
				continue
			}
			if err := printer.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, err))
			}
			continue
		}

		if _, err := o.Clientset.CoreV1().ReplicationControllers(rc.Namespace).Patch(rc.Name, types.StrategicMergePatchType, patches[0].Patch); err != nil {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, err))
			continue
		}
		printer, err := o.ToPrinter(fmt.Sprintf("retried rollout #%d", config.Status.LatestVersion))
		if err != nil {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, err))
			continue
		}
		if err := printer.PrintObj(info.Object, o.Out); err != nil {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, err))
			continue
		}
	}

	return utilerrors.NewAggregate(allErrs)
}
