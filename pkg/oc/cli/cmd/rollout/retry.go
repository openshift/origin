package rollout

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	"github.com/spf13/cobra"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/set"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

type RetryOptions struct {
	Mapper  meta.RESTMapper
	Typer   runtime.ObjectTyper
	Encoder runtime.Encoder
	Infos   []*resource.Info

	Out             io.Writer
	FilenameOptions resource.FilenameOptions

	Clientset kclientset.Interface
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

func NewCmdRolloutRetry(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &RetryOptions{}
	cmd := &cobra.Command{
		Use:     "retry (TYPE NAME | TYPE/NAME) [flags]",
		Long:    rolloutRetryLong,
		Example: fmt.Sprintf(rolloutRetryExample, fullName),
		Short:   "Retry the latest failed rollout",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(opts.Complete(f, cmd, out, args))
			kcmdutil.CheckErr(opts.Run())
		},
	}
	usage := "Filename, directory, or URL to a file identifying the resource to get from a server."
	kcmdutil.AddFilenameOptionFlags(cmd, &opts.FilenameOptions, usage)
	return cmd
}

func (o *RetryOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, out io.Writer, args []string) error {
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

func (o RetryOptions) Run() error {
	allErrs := []error{}
	mapping, err := o.Mapper.RESTMapping(kapi.Kind("ReplicationController"))
	if err != nil {
		return err
	}
	for _, info := range o.Infos {
		config, ok := info.Object.(*deployapi.DeploymentConfig)
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

		latestDeploymentName := deployutil.LatestDeploymentNameForConfig(config)
		rc, err := o.Clientset.Core().ReplicationControllers(config.Namespace).Get(latestDeploymentName, metav1.GetOptions{})
		if err != nil {
			if kerrors.IsNotFound(err) {
				allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("unable to find the latest rollout (#%d).\nYou can start a new rollout with 'oc rollout latest dc/%s'.", config.Status.LatestVersion, config.Name)))
				continue
			}
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, fmt.Errorf("unable to fetch replication controller %q", config.Name)))
			continue
		}

		if !deployutil.IsFailedDeployment(rc) {
			message := fmt.Sprintf("rollout #%d is %s; only failed deployments can be retried.\n", config.Status.LatestVersion, strings.ToLower(string(deployutil.DeploymentStatusFor(rc))))
			if deployutil.IsCompleteDeployment(rc) {
				message += fmt.Sprintf("You can start a new deployment with 'oc rollout latest dc/%s'.", config.Name)
			} else {
				message += fmt.Sprintf("Optionally, you can cancel this deployment with 'oc rollout cancel dc/%s'.", config.Name)
			}
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, errors.New(message)))
			continue
		}

		// Delete the deployer pod as well as the deployment hooks pods, if any
		pods, err := o.Clientset.Core().Pods(config.Namespace).List(metav1.ListOptions{LabelSelector: deployutil.DeployerPodSelector(latestDeploymentName).String()})
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

		patches := set.CalculatePatches([]*resource.Info{{Object: rc, Mapping: mapping}}, o.Encoder, func(*resource.Info) ([]byte, error) {
			rc.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
			delete(rc.Annotations, deployapi.DeploymentStatusReasonAnnotation)
			delete(rc.Annotations, deployapi.DeploymentCancelledAnnotation)
			return runtime.Encode(o.Encoder, rc)
		})

		if len(patches) == 0 {
			kcmdutil.PrintSuccess(o.Mapper, false, o.Out, info.Mapping.Resource, info.Name, false, "already retried")
			continue
		}

		if _, err := o.Clientset.Core().ReplicationControllers(rc.Namespace).Patch(rc.Name, types.StrategicMergePatchType, patches[0].Patch); err != nil {
			allErrs = append(allErrs, kcmdutil.AddSourceToErr("retrying", info.Source, err))
			continue
		}
		kcmdutil.PrintSuccess(o.Mapper, false, o.Out, info.Mapping.Resource, info.Name, false, fmt.Sprintf("retried rollout #%d", config.Status.LatestVersion))
	}

	return utilerrors.NewAggregate(allErrs)
}
