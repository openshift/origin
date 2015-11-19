package cmd

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"time"

	"github.com/docker/docker/pkg/units"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeployOptions holds all the options for the `deploy` command
type DeployOptions struct {
	out             io.Writer
	osClient        client.Interface
	kubeClient      kclient.Interface
	builder         *resource.Builder
	namespace       string
	baseCommandName string

	deploymentConfigName string
	deployLatest         bool
	retryDeploy          bool
	cancelDeploy         bool
	enableTriggers       bool
}

const (
	deployLong = `
View, start, cancel, or retry a deployment

This command allows you to control a deployment config. Each individual deployment is exposed
as a new replication controller, and the deployment process manages scaling down old deployments
and scaling up new ones. You can rollback to any previous deployment, or even scale multiple
deployments up at the same time.

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
When rolling back to a previous deployment, a new deployment will be created with an identical copy
of your config at the latest position.

If no options are given, shows information about the latest deployment.`

	deployExample = `  # Display the latest deployment for the 'database' deployment config
  $ %[1]s deploy database

  # Start a new deployment based on the 'database'
  $ %[1]s deploy database --latest

  # Retry the latest failed deployment based on 'frontend'
  # The deployer pod and any hook pods are deleted for the latest failed deployment
  $ %[1]s deploy frontend --retry

  # Cancel the in-progress deployment based on 'frontend'
  $ %[1]s deploy frontend --cancel`
)

// NewCmdDeploy creates a new `deploy` command.
func NewCmdDeploy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &DeployOptions{
		baseCommandName: fullName,
	}

	cmd := &cobra.Command{
		Use:        "deploy DEPLOYMENTCONFIG [--latest|--retry|--cancel|--enable-triggers]",
		Short:      "View, start, cancel, or retry a deployment",
		Long:       deployLong,
		Example:    fmt.Sprintf(deployExample, fullName),
		SuggestFor: []string{"deployment"},
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, out); err != nil {
				cmdutil.CheckErr(err)
			}

			if err := options.Validate(); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RunDeploy(); err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVar(&options.deployLatest, "latest", false, "Start a new deployment now.")
	cmd.Flags().BoolVar(&options.retryDeploy, "retry", false, "Retry the latest failed deployment.")
	cmd.Flags().BoolVar(&options.cancelDeploy, "cancel", false, "Cancel the in-progress deployment.")
	cmd.Flags().BoolVar(&options.enableTriggers, "enable-triggers", false, "Enables all image triggers for the deployment config.")

	return cmd
}

func (o *DeployOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	if len(args) > 1 {
		return errors.New("only one deployment config name is supported as argument.")
	}
	var err error

	o.osClient, o.kubeClient, err = f.Clients()
	if err != nil {
		return err
	}
	o.namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	o.builder = resource.NewBuilder(mapper, typer, f.ClientMapperForCommand())

	o.out = out

	if len(args) > 0 {
		o.deploymentConfigName = args[0]
	}

	return nil
}

func (o DeployOptions) Validate() error {
	if len(o.deploymentConfigName) == 0 {
		return errors.New("a deployment config name is required.")
	}
	numOptions := 0
	if o.deployLatest {
		numOptions++
	}
	if o.retryDeploy {
		numOptions++
	}
	if o.cancelDeploy {
		numOptions++
	}
	if o.enableTriggers {
		numOptions++
	}
	if numOptions > 1 {
		return errors.New("only one of --latest, --retry, --cancel, or --enable-triggers is allowed.")
	}
	return nil
}

func (o DeployOptions) RunDeploy() error {
	r := o.builder.
		NamespaceParam(o.namespace).
		ResourceNames("deploymentconfigs", o.deploymentConfigName).
		SingleResourceType().
		Do()
	resultObj, err := r.Object()
	if err != nil {
		return err
	}
	config, ok := resultObj.(*deployapi.DeploymentConfig)
	if !ok {
		return fmt.Errorf("%s is not a valid deployment config", o.deploymentConfigName)
	}

	switch {
	case o.deployLatest:
		err = o.deploy(config, o.out)
	case o.retryDeploy:
		err = o.retry(config, o.out)
	case o.cancelDeploy:
		err = o.cancel(config, o.out)
	case o.enableTriggers:
		err = o.reenableTriggers(config, o.out)
	default:
		describer := describe.NewLatestDeploymentsDescriber(o.osClient, o.kubeClient, -1)
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
func (o DeployOptions) deploy(config *deployapi.DeploymentConfig, out io.Writer) error {
	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := o.kubeClient.ReplicationControllers(config.Namespace).Get(deploymentName)
	if err == nil {
		// Reject attempts to start a concurrent deployment.
		status := deployutil.DeploymentStatusFor(deployment)
		if status != deployapi.DeploymentStatusComplete && status != deployapi.DeploymentStatusFailed {
			return fmt.Errorf("#%d is already in progress (%s).\nOptionally, you can cancel this deployment using the --cancel option.", config.LatestVersion, status)
		}
	} else {
		if !kerrors.IsNotFound(err) {
			return err
		}
	}

	config.LatestVersion++
	_, err = o.osClient.DeploymentConfigs(config.Namespace).Update(config)
	if err == nil {
		fmt.Fprintf(out, "Started deployment #%d\n", config.LatestVersion)
	}
	return err
}

// retry resets the status of the latest deployment to New, which will cause
// the deployment to be retried. An error is returned if the deployment is not
// currently in a failed state.
func (o DeployOptions) retry(config *deployapi.DeploymentConfig, out io.Writer) error {
	if config.LatestVersion == 0 {
		return fmt.Errorf("no deployments found for %s/%s", config.Namespace, config.Name)
	}
	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := o.kubeClient.ReplicationControllers(config.Namespace).Get(deploymentName)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return fmt.Errorf("unable to find the latest deployment (#%d).\nYou can start a new deployment using the --latest option.", config.LatestVersion)
		}
		return err
	}

	if status := deployutil.DeploymentStatusFor(deployment); status != deployapi.DeploymentStatusFailed {
		message := fmt.Sprintf("#%d is %s; only failed deployments can be retried.\n", config.LatestVersion, status)
		if status == deployapi.DeploymentStatusComplete {
			message += "You can start a new deployment using the --latest option."
		} else {
			message += "Optionally, you can cancel this deployment using the --cancel option."
		}

		return fmt.Errorf(message)
	}

	// Delete the deployer pod as well as the deployment hooks pods, if any
	pods, err := o.kubeClient.Pods(config.Namespace).List(deployutil.DeployerPodSelector(deploymentName), fields.Everything())
	if err != nil {
		return fmt.Errorf("failed to list deployer/hook pods for deployment #%d: %v", config.LatestVersion, err)
	}
	for _, pod := range pods.Items {
		err := o.kubeClient.Pods(pod.Namespace).Delete(pod.Name, kapi.NewDeleteOptions(0))
		if err != nil {
			return fmt.Errorf("failed to delete deployer/hook pod %s for deployment #%d: %v", pod.Name, config.LatestVersion, err)
		}
	}

	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	// clear out the cancellation flag as well as any previous status-reason annotation
	delete(deployment.Annotations, deployapi.DeploymentStatusReasonAnnotation)
	delete(deployment.Annotations, deployapi.DeploymentCancelledAnnotation)
	_, err = o.kubeClient.ReplicationControllers(deployment.Namespace).Update(deployment)
	if err == nil {
		fmt.Fprintf(out, "Retried #%d\n", config.LatestVersion)
	}
	return err
}

// cancel cancels any deployment process in progress for config.
func (o DeployOptions) cancel(config *deployapi.DeploymentConfig, out io.Writer) error {
	deployments, err := o.kubeClient.ReplicationControllers(config.Namespace).List(deployutil.ConfigSelector(config.Name), fields.Everything())
	if err != nil {
		return err
	}
	if len(deployments.Items) == 0 {
		fmt.Fprintf(out, "There have been no deployments for %s/%s\n", config.Namespace, config.Name)
		return nil
	}
	sort.Sort(deployutil.ByLatestVersionDesc(deployments.Items))
	failedCancellations := []string{}
	anyCancelled := false
	for _, deployment := range deployments.Items {
		status := deployutil.DeploymentStatusFor(&deployment)
		switch status {
		case deployapi.DeploymentStatusNew,
			deployapi.DeploymentStatusPending,
			deployapi.DeploymentStatusRunning:

			if deployutil.IsDeploymentCancelled(&deployment) {
				continue
			}

			deployment.Annotations[deployapi.DeploymentCancelledAnnotation] = deployapi.DeploymentCancelledAnnotationValue
			deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentCancelledByUser
			_, err := o.kubeClient.ReplicationControllers(deployment.Namespace).Update(&deployment)
			if err == nil {
				fmt.Fprintf(out, "Cancelled deployment #%d\n", config.LatestVersion)
				anyCancelled = true
			} else {
				fmt.Fprintf(out, "Couldn't cancel deployment #%d (status: %s): %v\n", deployutil.DeploymentVersionFor(&deployment), status, err)
				failedCancellations = append(failedCancellations, strconv.Itoa(deployutil.DeploymentVersionFor(&deployment)))
			}
		}
	}
	if len(failedCancellations) > 0 {
		return fmt.Errorf("couldn't cancel deployment %s", strings.Join(failedCancellations, ", "))
	}
	if !anyCancelled {
		latest := &deployments.Items[0]
		timeAt := strings.ToLower(units.HumanDuration(time.Now().Sub(latest.CreationTimestamp.Time)))
		fmt.Fprintf(out, "No deployments are in progress (latest deployment #%d %s %s ago)\n",
			deployutil.DeploymentVersionFor(latest),
			strings.ToLower(string(deployutil.DeploymentStatusFor(latest))),
			timeAt)
	}
	return nil
}

// reenableTriggers enables all image triggers and then persists config.
func (o DeployOptions) reenableTriggers(config *deployapi.DeploymentConfig, out io.Writer) error {
	enabled := []string{}
	for _, trigger := range config.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
			trigger.ImageChangeParams.Automatic = true
			enabled = append(enabled, trigger.ImageChangeParams.From.Name)
		}
	}
	if len(enabled) == 0 {
		fmt.Fprintln(out, "No image triggers found to enable")
		return nil
	}
	_, err := o.osClient.DeploymentConfigs(config.Namespace).Update(config)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Enabled image triggers: %s\n", strings.Join(enabled, ","))
	return nil
}
