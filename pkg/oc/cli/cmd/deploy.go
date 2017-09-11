package cmd

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"time"

	units "github.com/docker/go-units"
	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsinternalclient "github.com/openshift/origin/pkg/apps/client/internalversion"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

// DeployOptions holds all the options for the `deploy` command
type DeployOptions struct {
	out             io.Writer
	appsClient      appsclient.AppsInterface
	kubeClient      kclientset.Interface
	builder         *resource.Builder
	namespace       string
	baseCommandName string

	deploymentConfigName string
	deployLatest         bool
	retryDeploy          bool
	cancelDeploy         bool
	enableTriggers       bool
	follow               bool
}

var (
	deployLong = templates.LongDesc(`
		View, start, cancel, or retry a deployment

		This command allows you to control a deployment config. Each individual deployment is exposed
		as a new replication controller, and the deployment process manages scaling down old deployments
		and scaling up new ones. Use '%[1]s rollback' to rollback to any previous deployment.

		There are several deployment strategies defined:

		* Rolling (default) - scales up the new deployment in stages, gradually reducing the number
		  of old deployments. If one of the new deployed pods never becomes "ready", the new deployment
		  will be rolled back (scaled down to zero). Use when your application can tolerate two versions
		  of code running at the same time (many web applications, scalable databases)
		* Recreate - scales the old deployment down to zero, then scales the new deployment up to full.
		  Use when your application cannot tolerate two versions of code running at the same time
		* Custom - run your own deployment process inside a Docker container using your own scripts.

		If a deployment fails, you may opt to retry it (if the error was transient). Some deployments may
		never successfully complete - in which case you can use the '--latest' flag to force a redeployment.
		If a deployment config has completed deploying successfully at least once in the past, it would be
		automatically rolled back in the event of a new failed deployment. Note that you would still need
		to update the erroneous deployment config in order to have its template persisted across your
		application.

		If you want to cancel a running deployment, use '--cancel' but keep in mind that this is a best-effort
		operation and may take some time to complete. It’s possible the deployment will partially or totally
		complete before the cancellation is effective. In such a case an appropriate event will be emitted.

		If no options are given, shows information about the latest deployment.`)

	deployExample = templates.Examples(`
		# Display the latest deployment for the 'database' deployment config
	  %[1]s deploy database

	  # Start a new deployment based on the 'database'
	  %[1]s deploy database --latest

	  # Start a new deployment and follow its log
	  %[1]s deploy database --latest --follow

	  # Retry the latest failed deployment based on 'frontend'
	  # The deployer pod and any hook pods are deleted for the latest failed deployment
	  %[1]s deploy frontend --retry

	  # Cancel the in-progress deployment based on 'frontend'
	  %[1]s deploy frontend --cancel`)
)

// NewCmdDeploy creates a new `deploy` command.
func NewCmdDeploy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &DeployOptions{
		baseCommandName: fullName,
	}

	cmd := &cobra.Command{
		Use:        "deploy DEPLOYMENTCONFIG [--latest|--retry|--cancel|--enable-triggers]",
		Short:      "View, start, cancel, or retry a deployment",
		Long:       fmt.Sprintf(deployLong, fullName),
		Example:    fmt.Sprintf(deployExample, fullName),
		SuggestFor: []string{"deployment"},
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RunDeploy(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}
	cmd.Deprecated = "Use the `rollout latest` and `rollout cancel` commands instead."

	cmd.Flags().BoolVar(&options.deployLatest, "latest", false, "If true, start a new deployment now.")
	cmd.Flags().MarkDeprecated("latest", fmt.Sprintf("use '%s rollout latest' instead", fullName))
	cmd.Flags().BoolVar(&options.retryDeploy, "retry", false, "If true, retry the latest failed deployment.")
	cmd.Flags().BoolVar(&options.cancelDeploy, "cancel", false, "If true, cancel the in-progress deployment.")
	cmd.Flags().MarkDeprecated("cancel", fmt.Sprintf("use '%s rollout cancel' instead", fullName))
	cmd.Flags().BoolVar(&options.enableTriggers, "enable-triggers", false, "If true, enables all image triggers for the deployment config.")
	cmd.Flags().MarkDeprecated("enable-triggers", fmt.Sprintf("use '%s set triggers' instead", fullName))
	cmd.Flags().BoolVar(&options.follow, "follow", false, "If true, follow the logs of a deployment")

	return cmd
}

func (o *DeployOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	if len(args) > 1 {
		return errors.New("only one deployment config name is supported as argument.")
	}
	var err error

	o.kubeClient, err = f.ClientSet()
	if err != nil {
		return err
	}
	client, err := f.OpenshiftInternalAppsClient()
	if err != nil {
		return err
	}
	o.appsClient = client.Apps()
	o.namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.builder = f.NewBuilder()
	o.out = out

	if len(args) > 0 {
		o.deploymentConfigName = args[0]
	}

	return nil
}

func (o DeployOptions) Validate() error {
	if len(o.deploymentConfigName) == 0 {
		msg := fmt.Sprintf("a deployment config name is required.\nUse \"%s get dc\" for a list of available deployment configs.", o.baseCommandName)
		return errors.New(msg)
	}
	numOptions := 0
	if o.deployLatest {
		numOptions++
	}
	if o.retryDeploy {
		numOptions++
	}
	if o.cancelDeploy {
		if o.follow {
			return errors.New("cannot follow the logs while canceling a deployment")
		}
		numOptions++
	}
	if o.enableTriggers {
		if o.follow {
			return errors.New("cannot follow the logs while enabling triggers for a deployment")
		}
		numOptions++
	}
	if numOptions > 1 {
		return errors.New("only one of --latest, --retry, --cancel, or --enable-triggers is allowed.")
	}
	return nil
}

func (o DeployOptions) RunDeploy() error {
	r := o.builder.
		Internal().
		NamespaceParam(o.namespace).
		ResourceNames("deploymentconfigs", o.deploymentConfigName).
		SingleResourceType().
		Do()
	resultObj, err := r.Object()
	if err != nil {
		return err
	}
	config, ok := resultObj.(*appsapi.DeploymentConfig)
	if !ok {
		return fmt.Errorf("%s is not a valid deployment config", o.deploymentConfigName)
	}

	switch {
	case o.deployLatest:
		err = o.deploy(config)
	case o.retryDeploy:
		err = o.retry(config)
	case o.cancelDeploy:
		err = o.cancel(config)
	case o.enableTriggers:
		err = o.reenableTriggers(config)
	default:
		if o.follow {
			return o.getLogs(config)
		}
		describer := describe.NewLatestDeploymentsDescriber(o.appsClient, o.kubeClient, -1)
		desc, err := describer.Describe(config.Namespace, config.Name)
		if err != nil {
			return err
		}
		fmt.Fprint(o.out, desc)
	}

	return err
}

// deploy launches a new deployment unless there's already a deployment
// process in progress for config.
func (o DeployOptions) deploy(config *appsapi.DeploymentConfig) error {
	if config.Spec.Paused {
		return fmt.Errorf("cannot deploy a paused deployment config")
	}
	// TODO: This implies that deploymentconfig.status.latestVersion is always synced. Currently,
	// that's the case because clients (oc, trigger controllers) are updating the status directly.
	// Clients should be acting either on spec or on annotations and status updates should be a
	// responsibility of the main controller. We need to start by unplugging this assumption from
	// our client tools.
	deploymentName := appsutil.LatestDeploymentNameForConfig(config)
	deployment, err := o.kubeClient.Core().ReplicationControllers(config.Namespace).Get(deploymentName, metav1.GetOptions{})
	if err == nil && !appsutil.IsTerminatedDeployment(deployment) {
		// Reject attempts to start a concurrent deployment.
		return fmt.Errorf("#%d is already in progress (%s).\nOptionally, you can cancel this deployment using 'oc rollout cancel dc/%s'.",
			config.Status.LatestVersion, appsutil.DeploymentStatusFor(deployment), config.Name)
	}
	if err != nil && !kerrors.IsNotFound(err) {
		return err
	}

	request := &appsapi.DeploymentRequest{
		Name:   config.Name,
		Latest: false,
		Force:  true,
	}

	dc, err := o.appsClient.DeploymentConfigs(config.Namespace).Instantiate(config.Name, request)
	// Pre 1.4 servers don't support the instantiate endpoint. Fallback to incrementing
	// latestVersion on them.
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		config.Status.LatestVersion++
		dc, err = o.appsClient.DeploymentConfigs(config.Namespace).Update(config)
	}
	if err != nil {
		if kerrors.IsBadRequest(err) {
			err = fmt.Errorf("%v - try 'oc rollout latest dc/%s'", err, config.Name)
		}
		return err
	}
	fmt.Fprintf(o.out, "Started deployment #%d\n", dc.Status.LatestVersion)
	if o.follow {
		return o.getLogs(dc)
	}
	fmt.Fprintf(o.out, "Use '%s logs -f dc/%s' to track its progress.\n", o.baseCommandName, dc.Name)
	return nil
}

// retry resets the status of the latest deployment to New, which will cause
// the deployment to be retried. An error is returned if the deployment is not
// currently in a failed state.
func (o DeployOptions) retry(config *appsapi.DeploymentConfig) error {
	if config.Spec.Paused {
		return fmt.Errorf("cannot retry a paused deployment config")
	}
	if config.Status.LatestVersion == 0 {
		return fmt.Errorf("no deployments found for %s/%s", config.Namespace, config.Name)
	}
	// TODO: This implies that deploymentconfig.status.latestVersion is always synced. Currently,
	// that's the case because clients (oc, trigger controllers) are updating the status directly.
	// Clients should be acting either on spec or on annotations and status updates should be a
	// responsibility of the main controller. We need to start by unplugging this assumption from
	// our client tools.
	deploymentName := appsutil.LatestDeploymentNameForConfig(config)
	deployment, err := o.kubeClient.Core().ReplicationControllers(config.Namespace).Get(deploymentName, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			return fmt.Errorf("unable to find the latest deployment (#%d).\nYou can start a new deployment with 'oc deploy --latest dc/%s'.", config.Status.LatestVersion, config.Name)
		}
		return err
	}

	if !appsutil.IsFailedDeployment(deployment) {
		message := fmt.Sprintf("#%d is %s; only failed deployments can be retried.\n", config.Status.LatestVersion, appsutil.DeploymentStatusFor(deployment))
		if appsutil.IsCompleteDeployment(deployment) {
			message += fmt.Sprintf("You can start a new deployment with 'oc deploy --latest dc/%s'.", config.Name)
		} else {
			message += fmt.Sprintf("Optionally, you can cancel this deployment with 'oc rollout cancel dc/%s'.", config.Name)
		}

		return fmt.Errorf(message)
	}

	// Delete the deployer pod as well as the deployment hooks pods, if any
	pods, err := o.kubeClient.Core().Pods(config.Namespace).List(metav1.ListOptions{LabelSelector: appsutil.DeployerPodSelector(deploymentName).String()})
	if err != nil {
		return fmt.Errorf("failed to list deployer/hook pods for deployment #%d: %v", config.Status.LatestVersion, err)
	}
	for _, pod := range pods.Items {
		err := o.kubeClient.Core().Pods(pod.Namespace).Delete(pod.Name, metav1.NewDeleteOptions(0))
		if err != nil {
			return fmt.Errorf("failed to delete deployer/hook pod %s for deployment #%d: %v", pod.Name, config.Status.LatestVersion, err)
		}
	}

	deployment.Annotations[appsapi.DeploymentStatusAnnotation] = string(appsapi.DeploymentStatusNew)
	// clear out the cancellation flag as well as any previous status-reason annotation
	delete(deployment.Annotations, appsapi.DeploymentStatusReasonAnnotation)
	delete(deployment.Annotations, appsapi.DeploymentCancelledAnnotation)
	_, err = o.kubeClient.Core().ReplicationControllers(deployment.Namespace).Update(deployment)
	if err != nil {
		return err
	}
	fmt.Fprintf(o.out, "Retried #%d\n", config.Status.LatestVersion)
	if o.follow {
		return o.getLogs(config)
	}
	fmt.Fprintf(o.out, "Use '%s logs -f dc/%s' to track its progress.\n", o.baseCommandName, config.Name)
	return nil
}

// cancel cancels any deployment process in progress for config.
// TODO: this code will be deprecated
func (o DeployOptions) cancel(config *appsapi.DeploymentConfig) error {
	if config.Spec.Paused {
		return fmt.Errorf("cannot cancel a paused deployment config")
	}
	deploymentList, err := o.kubeClient.Core().ReplicationControllers(config.Namespace).List(metav1.ListOptions{LabelSelector: appsutil.ConfigSelector(config.Name).String()})
	if err != nil {
		return err
	}
	if len(deploymentList.Items) == 0 {
		fmt.Fprintf(o.out, "There have been no deployments for %s/%s\n", config.Namespace, config.Name)
		return nil
	}
	deployments := make([]*kapi.ReplicationController, 0, len(deploymentList.Items))
	for i := range deploymentList.Items {
		deployments = append(deployments, &deploymentList.Items[i])
	}
	sort.Sort(appsutil.ByLatestVersionDesc(deployments))
	failedCancellations := []string{}
	anyCancelled := false
	for _, deployment := range deployments {
		status := appsutil.DeploymentStatusFor(deployment)
		switch status {
		case appsapi.DeploymentStatusNew,
			appsapi.DeploymentStatusPending,
			appsapi.DeploymentStatusRunning:

			if appsutil.IsDeploymentCancelled(deployment) {
				continue
			}

			deployment.Annotations[appsapi.DeploymentCancelledAnnotation] = appsapi.DeploymentCancelledAnnotationValue
			deployment.Annotations[appsapi.DeploymentStatusReasonAnnotation] = appsapi.DeploymentCancelledByUser
			_, err := o.kubeClient.Core().ReplicationControllers(deployment.Namespace).Update(deployment)
			if err == nil {
				fmt.Fprintf(o.out, "Cancelled deployment #%d\n", config.Status.LatestVersion)
				anyCancelled = true
			} else {
				fmt.Fprintf(o.out, "Couldn't cancel deployment #%d (status: %s): %v\n", appsutil.DeploymentVersionFor(deployment), status, err)
				failedCancellations = append(failedCancellations, strconv.FormatInt(appsutil.DeploymentVersionFor(deployment), 10))
			}
		}
	}
	if len(failedCancellations) > 0 {
		return fmt.Errorf("couldn't cancel deployment %s", strings.Join(failedCancellations, ", "))
	}
	if !anyCancelled {
		latest := deployments[0]
		maybeCancelling := ""
		if appsutil.IsDeploymentCancelled(latest) && !appsutil.IsTerminatedDeployment(latest) {
			maybeCancelling = " (cancelling)"
		}
		timeAt := strings.ToLower(units.HumanDuration(time.Now().Sub(latest.CreationTimestamp.Time)))
		fmt.Fprintf(o.out, "No deployments are in progress (latest deployment #%d %s%s %s ago)\n",
			appsutil.DeploymentVersionFor(latest),
			strings.ToLower(string(appsutil.DeploymentStatusFor(latest))),
			maybeCancelling,
			timeAt)
	}
	return nil
}

// reenableTriggers enables all image triggers and then persists config.
func (o DeployOptions) reenableTriggers(config *appsapi.DeploymentConfig) error {
	enabled := []string{}
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == appsapi.DeploymentTriggerOnImageChange {
			trigger.ImageChangeParams.Automatic = true
			enabled = append(enabled, trigger.ImageChangeParams.From.Name)
		}
	}
	if len(enabled) == 0 {
		fmt.Fprintln(o.out, "No image triggers found to enable")
		return nil
	}
	_, err := o.appsClient.DeploymentConfigs(config.Namespace).Update(config)
	if err != nil {
		return err
	}
	fmt.Fprintf(o.out, "Enabled image triggers: %s\n", strings.Join(enabled, ","))
	return nil
}

func (o DeployOptions) getLogs(config *appsapi.DeploymentConfig) error {
	opts := appsapi.DeploymentLogOptions{
		Follow: true,
	}
	logClient := appsinternalclient.NewRolloutLogClient(o.appsClient.RESTClient(), config.Namespace)
	readCloser, err := logClient.Logs(config.Name, opts).Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()
	_, err = io.Copy(o.out, readCloser)
	return err
}
