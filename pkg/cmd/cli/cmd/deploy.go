package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

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
}

const (
	deploy_long = `View, start and restart deployments.

If no options are given, view the latest deployment.

NOTE: This command is still under active development and is subject to change.`

	deploy_example = `  // Display the latest deployment for the 'database' deployment config
  $ %[1]s deploy database

  // Start a new deployment based on the 'database' deployment config
  $ %[1]s deploy frontend --latest

  // Retry the latest failed deployment based on the 'frontend' deployment config
  $ %[1]s deploy frontend --retry`
)

// NewCmdDeploy creates a new `deploy` command.
func NewCmdDeploy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &DeployOptions{
		baseCommandName: fullName,
	}

	cmd := &cobra.Command{
		Use:     "deploy DEPLOYMENTCONFIG",
		Short:   "View, start and restart deployments.",
		Long:    deploy_long,
		Example: fmt.Sprintf(deploy_example, fullName),
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
		return errors.New("A deploymentConfig name is required.")
	}
	if len(args) > 1 {
		return errors.New("Only one deploymentConfig name is supported as argument.")
	}
	if o.deployLatest && o.retryDeploy {
		return errors.New("Only one of --latest or --retry is allowed.")
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

	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	_, err = c.client.UpdateDeployment(deployment)
	if err == nil {
		fmt.Fprintf(out, "retried #%d\n", config.LatestVersion)
	}
	return err
}

// deployCommandClientImpl is a pluggable deployCommandClient.
type deployCommandClientImpl struct {
	GetDeploymentFn          func(namespace, name string) (*kapi.ReplicationController, error)
	UpdateDeploymentConfigFn func(*deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error)
	UpdateDeploymentFn       func(*kapi.ReplicationController) (*kapi.ReplicationController, error)
}

func (c *deployCommandClientImpl) GetDeployment(namespace, name string) (*kapi.ReplicationController, error) {
	return c.GetDeploymentFn(namespace, name)
}
func (c *deployCommandClientImpl) UpdateDeploymentConfig(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	return c.UpdateDeploymentConfigFn(config)
}

func (c *deployCommandClientImpl) UpdateDeployment(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return c.UpdateDeploymentFn(deployment)
}
