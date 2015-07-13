package cmd

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

type DeployOptions struct {
	out             io.Writer
	osClient        *client.Client
	kubeClient      *kclient.Client
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

	deployExample = `  // Display the latest deployment for the 'database' deployment config
  $ %[1]s deploy database

  // Start a new deployment based on the 'database'
  $ %[1]s deploy database --latest

  // Retry the latest failed deployment based on 'frontend'
  // The deployer pod and any hook pods are deleted for the latest failed deployment
  $ %[1]s deploy frontend --retry

  // Cancel the in-progress deployment based on 'frontend'
  $ %[1]s deploy frontend --cancel`
)

// NewCmdDeploy creates a new `deploy` command.
func NewCmdDeploy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &DeployOptions{
		baseCommandName: fullName,
	}

	cmd := &cobra.Command{
		Use:     "deploy DEPLOYMENTCONFIG",
		Short:   "View, start, cancel, or retry a deployment",
		Long:    deployLong,
		Example: fmt.Sprintf(deployExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, args, out); err != nil {
				cmdutil.CheckErr(err)
			}

			if err := options.Validate(args); err != nil {
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
	var err error

	o.osClient, o.kubeClient, err = f.Clients()
	if err != nil {
		return err
	}
	o.namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.out = out

	if len(args) > 0 {
		o.deploymentConfigName = args[0]
	}

	return nil
}

func (o DeployOptions) Validate(args []string) error {
	if len(args) == 0 || len(args[0]) == 0 {
		return errors.New("a DeploymentConfig name is required.")
	}
	if len(args) > 1 {
		return errors.New("only one DeploymentConfig name is supported as argument.")
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
	config, err := o.osClient.DeploymentConfigs(o.namespace).Get(o.deploymentConfigName)
	if err != nil {
		return err
	}

	commandClient := &deployCommandClientImpl{
		GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
			return o.kubeClient.ReplicationControllers(namespace).Get(name)
		},
		ListDeploymentsForConfigFn: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
			list, err := o.kubeClient.ReplicationControllers(namespace).List(deployutil.ConfigSelector(configName))
			if err != nil {
				return nil, err
			}
			return list, nil
		},

		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			return o.osClient.DeploymentConfigs(config.Namespace).Update(config)
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			return o.kubeClient.ReplicationControllers(deployment.Namespace).Update(deployment)
		},

		ListDeployerPodsForFn: func(namespace, deploymentName string) (*kapi.PodList, error) {
			selector, err := labels.Parse(fmt.Sprintf("%s=%s", deployapi.DeployerPodForDeploymentLabel, deploymentName))
			if err != nil {
				return nil, err
			}
			return o.kubeClient.Pods(namespace).List(selector, fields.Everything())
		},
		DeletePodFn: func(pod *kapi.Pod) error {
			return o.kubeClient.Pods(pod.Namespace).Delete(pod.Name, nil)
		},
	}

	switch {
	case o.deployLatest:
		c := &deployLatestCommand{client: commandClient}
		err = c.deploy(config, o.out)
	case o.retryDeploy:
		c := &retryDeploymentCommand{client: commandClient}
		err = c.retry(config, o.out)
	case o.cancelDeploy:
		c := &cancelDeploymentCommand{client: commandClient}
		err = c.cancel(config, o.out)
	case o.enableTriggers:
		t := &triggerEnabler{
			updateConfig: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				return o.osClient.DeploymentConfigs(namespace).Update(config)
			},
		}
		err = t.enableTriggers(config, o.out)
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

// deployCommandClient abstracts access to the API server.
type deployCommandClient interface {
	GetDeployment(namespace, name string) (*kapi.ReplicationController, error)
	ListDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error)
	UpdateDeploymentConfig(*deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	UpdateDeployment(*kapi.ReplicationController) (*kapi.ReplicationController, error)

	ListDeployerPodsFor(namespace, deploymentName string) (*kapi.PodList, error)
	DeletePod(pod *kapi.Pod) error
}

// deployLatestCommand can launch new deployments.
type deployLatestCommand struct {
	client deployCommandClient
}

// deploy launches a new deployment unless there's already a deployment
// process in progress for config.
func (c *deployLatestCommand) deploy(config *deployapi.DeploymentConfig, out io.Writer) error {
	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.client.GetDeployment(config.Namespace, deploymentName)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
	} else {
		// Reject attempts to start a concurrent deployment.
		status := deployutil.DeploymentStatusFor(deployment)
		if status != deployapi.DeploymentStatusComplete && status != deployapi.DeploymentStatusFailed {
			return fmt.Errorf("#%d is already in progress (%s).\nOptionally, you can cancel this deployment using the --cancel option.", config.LatestVersion, status)
		}
	}

	config.LatestVersion++
	_, err = c.client.UpdateDeploymentConfig(config)
	if err == nil {
		fmt.Fprintf(out, "Started deployment #%d\n", config.LatestVersion)
	}
	return err
}

// retryDeploymentCommand can retry failed deployments.
type retryDeploymentCommand struct {
	client deployCommandClient
}

// retry resets the status of the latest deployment to New, which will cause
// the deployment to be retried. An error is returned if the deployment is not
// currently in a failed state.
func (c *retryDeploymentCommand) retry(config *deployapi.DeploymentConfig, out io.Writer) error {
	if config.LatestVersion == 0 {
		return fmt.Errorf("no deployments found for %s/%s", config.Namespace, config.Name)
	}
	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.client.GetDeployment(config.Namespace, deploymentName)
	if err != nil {
		if kerrors.IsNotFound(err) {
			return fmt.Errorf("Unable to find the latest deployment (#%d).\nYou can start a new deployment using the --latest option.", config.LatestVersion)
		}
		return err
	}

	if status := deployutil.DeploymentStatusFor(deployment); status != deployapi.DeploymentStatusFailed {
		message := fmt.Sprintf("#%d is %s; only failed deployments can be retried.\n", config.LatestVersion, status)
		if status == deployapi.DeploymentStatusComplete {
			message += fmt.Sprintf("You can start a new deployment using the --latest option.")
		} else {
			message += fmt.Sprintf("Optionally, you can cancel this deployment using the --cancel option.", config.LatestVersion)
		}

		return fmt.Errorf(message)
	}

	// Delete the deployer pod as well as the deployment hooks pods, if any
	pods, err := c.client.ListDeployerPodsFor(config.Namespace, deploymentName)
	if err != nil {
		return fmt.Errorf("Failed to list deployer/hook pods for deployment #%d: %v", config.LatestVersion, err)
	}
	for _, pod := range pods.Items {
		err := c.client.DeletePod(&pod)
		if err != nil {
			return fmt.Errorf("Failed to delete deployer/hook pod %s for deployment #%d: %v", pod.Name, config.LatestVersion, err)
		}
	}

	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	// clear out the cancellation flag as well as any previous status-reason annotation
	delete(deployment.Annotations, deployapi.DeploymentStatusReasonAnnotation)
	delete(deployment.Annotations, deployapi.DeploymentCancelledAnnotation)
	_, err = c.client.UpdateDeployment(deployment)
	if err == nil {
		fmt.Fprintf(out, "retried #%d\n", config.LatestVersion)
	}
	return err
}

// cancelDeploymentCommand cancels the in-progress deployments.
type cancelDeploymentCommand struct {
	client deployCommandClient
}

// cancel cancels any deployment process in progress for config.
func (c *cancelDeploymentCommand) cancel(config *deployapi.DeploymentConfig, out io.Writer) error {
	deployments, err := c.client.ListDeploymentsForConfig(config.Namespace, config.Name)
	if err != nil {
		return err
	}
	if len(deployments.Items) == 0 {
		fmt.Fprintln(out, "no deployments found to cancel")
		return nil
	}
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
			_, err := c.client.UpdateDeployment(&deployment)
			if err == nil {
				fmt.Fprintf(out, "cancelled deployment #%d\n", config.LatestVersion)
				anyCancelled = true
			} else {
				fmt.Fprintf(out, "couldn't cancel deployment #%d (status: %s): %v\n", deployutil.DeploymentVersionFor(&deployment), status, err)
				failedCancellations = append(failedCancellations, strconv.Itoa(deployutil.DeploymentVersionFor(&deployment)))
			}
		}
	}
	if len(failedCancellations) > 0 {
		return fmt.Errorf("couldn't cancel deployment %s", strings.Join(failedCancellations, ", "))
	}
	if !anyCancelled {
		fmt.Fprintln(out, "no active deployments to cancel")
	}
	return nil
}

// triggerEnabler can enable image triggers for a config.
type triggerEnabler struct {
	// updateConfig persists config.
	updateConfig func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
}

// enableTriggers enables all image triggers and then persists config.
func (t *triggerEnabler) enableTriggers(config *deployapi.DeploymentConfig, out io.Writer) error {
	enabled := []string{}
	for _, trigger := range config.Triggers {
		if trigger.Type == deployapi.DeploymentTriggerOnImageChange {
			trigger.ImageChangeParams.Automatic = true
			enabled = append(enabled, trigger.ImageChangeParams.From.Name)
		}
	}
	if len(enabled) == 0 {
		fmt.Fprintln(out, "no image triggers found to enable")
		return nil
	}
	_, err := t.updateConfig(config.Namespace, config)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "enabled image triggers: %s\n", strings.Join(enabled, ","))
	return nil
}

// deployCommandClientImpl is a pluggable deployCommandClient.
type deployCommandClientImpl struct {
	GetDeploymentFn            func(namespace, name string) (*kapi.ReplicationController, error)
	ListDeploymentsForConfigFn func(namespace, configName string) (*kapi.ReplicationControllerList, error)
	UpdateDeploymentConfigFn   func(*deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentFn         func(*kapi.ReplicationController) (*kapi.ReplicationController, error)

	ListDeployerPodsForFn func(namespace, deploymentName string) (*kapi.PodList, error)
	DeletePodFn           func(pod *kapi.Pod) error
}

func (c *deployCommandClientImpl) GetDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return c.GetDeploymentFn(namespace, name)
}

func (c *deployCommandClientImpl) ListDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error) {
	return c.ListDeploymentsForConfigFn(namespace, configName)
}

func (c *deployCommandClientImpl) UpdateDeploymentConfig(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return c.UpdateDeploymentConfigFn(config)
}

func (c *deployCommandClientImpl) UpdateDeployment(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return c.UpdateDeploymentFn(deployment)
}

func (c *deployCommandClientImpl) ListDeployerPodsFor(namespace, deploymentName string) (*kapi.PodList, error) {
	return c.ListDeployerPodsForFn(namespace, deploymentName)
}

func (c *deployCommandClientImpl) DeletePod(pod *kapi.Pod) error {
	return c.DeletePodFn(pod)
}
