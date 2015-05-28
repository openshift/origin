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
}

const (
	deployLong = `View, start and restart deployments.

If no options are given, view the latest deployment.

NOTE: This command is still under active development and is subject to change.`

	deployExample = `  // Display the latest deployment for the 'database' DeploymentConfig
  $ %[1]s deploy database

  // Start a new deployment based on the 'database' DeploymentConfig
  $ %[1]s deploy frontend --latest

  // Retry the latest failed deployment based on the 'frontend' DeploymentConfig
  $ %[1]s deploy frontend --retry

  // Cancel the in-progress deployment based on the 'frontend' DeploymentConfig
  $ %[1]s deploy frontend --cancel`
)

// NewCmdDeploy creates a new `deploy` command.
func NewCmdDeploy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &DeployOptions{
		baseCommandName: fullName,
	}

	cmd := &cobra.Command{
		Use:     "deploy DEPLOYMENTCONFIG",
		Short:   "View, start and restart deployments.",
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

	return cmd
}

func (o *DeployOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	var err error

	o.osClient, o.kubeClient, err = f.Clients()
	if err != nil {
		return err
	}
	o.namespace, err = f.DefaultNamespace()
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
	if numOptions > 1 {
		return errors.New("only one of --latest, --retry, or --cancel is allowed.")
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
			selector, err := labels.Parse(fmt.Sprintf("%s=%s", deployapi.DeploymentConfigLabel, configName))
			if err != nil {
				return nil, err
			}
			return o.kubeClient.ReplicationControllers(namespace).List(selector)
		},

		UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
			return o.osClient.DeploymentConfigs(config.Namespace).Update(config)
		},
		UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
			return o.kubeClient.ReplicationControllers(deployment.Namespace).Update(deployment)
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
	default:
		describer := describe.NewLatestDeploymentsDescriber(o.osClient, o.kubeClient, -1)
		desc, err := describer.Describe(config.Namespace, config.Name)
		if err != nil {
			return err
		}
		fmt.Fprintln(o.out, desc)
	}

	return err
}

// deployCommandClient abstracts access to the API server.
type deployCommandClient interface {
	GetDeployment(namespace, name string) (*kapi.ReplicationController, error)
	ListDeploymentsForConfig(namespace, configName string) (*kapi.ReplicationControllerList, error)
	UpdateDeploymentConfig(*deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	UpdateDeployment(*kapi.ReplicationController) (*kapi.ReplicationController, error)
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
			return fmt.Errorf("#%d is already in progress (%s)", config.LatestVersion, status)
		}
	}

	config.LatestVersion++
	_, err = c.client.UpdateDeploymentConfig(config)
	if err == nil {
		fmt.Fprintf(out, "deployed #%d\n", config.LatestVersion)
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
		return fmt.Errorf("no failed deployments found for %s/%s", config.Namespace, config.Name)
	}
	deploymentName := deployutil.LatestDeploymentNameForConfig(config)
	deployment, err := c.client.GetDeployment(config.Namespace, deploymentName)
	if err != nil {
		return err
	}

	if status := deployutil.DeploymentStatusFor(deployment); status != deployapi.DeploymentStatusFailed {
		return fmt.Errorf("#%d is %s; only failed deployments can be retried", config.LatestVersion, status)
	}

	previousVersion := config.LatestVersion
	config.LatestVersion++
	_, err = c.client.UpdateDeploymentConfig(config)
	if err == nil {
		fmt.Fprintf(out, "deployed #%d (retry of #%d)\n", config.LatestVersion, previousVersion)
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
	failedCancellations := []string{}
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
				fmt.Fprintf(out, "cancelled #%d\n", config.LatestVersion)
			} else {
				fmt.Fprintf(out, "couldn't cancel deployment %d (status: %s): %v", deployutil.DeploymentVersionFor(&deployment), status, err)
				failedCancellations = append(failedCancellations, strconv.Itoa(deployutil.DeploymentVersionFor(&deployment)))
			}
		}
	}
	if len(failedCancellations) == 0 {
		return nil
	} else {
		return fmt.Errorf("couldn't cancel deployment %s", strings.Join(failedCancellations, ", "))
	}
}

// deployCommandClientImpl is a pluggable deployCommandClient.
type deployCommandClientImpl struct {
	GetDeploymentFn            func(namespace, name string) (*kapi.ReplicationController, error)
	ListDeploymentsForConfigFn func(namespace, configName string) (*kapi.ReplicationControllerList, error)
	UpdateDeploymentConfigFn   func(*deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentFn         func(*kapi.ReplicationController) (*kapi.ReplicationController, error)
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
