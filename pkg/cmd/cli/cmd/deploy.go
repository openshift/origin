package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

const newCmdDeployDescription = `
View, start and restart deployments.

If no options are given, view the latest deployment.

NOTE: This command is still under active development and is subject to change.
`

// NewCmdDeploy creates a new `deploy` command.
func NewCmdDeploy(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	var deployLatest bool
	var retryDeploy bool

	cmd := &cobra.Command{
		Use:   "deploy <deploymentConfig>",
		Short: "View, start and restart deployments.",
		Long:  newCmdDeployDescription,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 || len(args[0]) == 0 {
				fmt.Println(cmdutil.UsageError(cmd, "A deploymentConfig name is required."))
				return
			}
			if deployLatest && retryDeploy {
				fmt.Println(cmdutil.UsageError(cmd, "Only one of --latest or --retry is allowed."))
				return
			}

			configName := args[0]

			osClient, kubeClient, err := f.Clients()
			cmdutil.CheckErr(err)

			namespace, err := f.DefaultNamespace()
			cmdutil.CheckErr(err)

			config, err := osClient.DeploymentConfigs(namespace).Get(configName)
			cmdutil.CheckErr(err)

			commandClient := &deployCommandClientImpl{
				GetDeploymentFn: func(namespace, name string) (*kapi.ReplicationController, error) {
					return kubeClient.ReplicationControllers(namespace).Get(name)
				},
				UpdateDeploymentConfigFn: func(config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
					return osClient.DeploymentConfigs(config.Namespace).Update(config)
				},
				UpdateDeploymentFn: func(deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
					return kubeClient.ReplicationControllers(deployment.Namespace).Update(deployment)
				},
			}

			switch {
			case deployLatest:
				c := &deployLatestCommand{client: commandClient}
				err = c.deploy(config, out)
			case retryDeploy:
				c := &retryDeploymentCommand{client: commandClient}
				err = c.retry(config, out)
			default:
				describer := describe.NewLatestDeploymentDescriber(osClient, kubeClient)
				desc, err := describer.Describe(config.Namespace, config.Name)
				cmdutil.CheckErr(err)
				fmt.Fprintln(out, desc)
			}
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVar(&deployLatest, "latest", false, "Start a new deployment now.")
	cmd.Flags().BoolVar(&retryDeploy, "retry", false, "Retry the latest failed deployment.")

	return cmd
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
		status := statusFor(deployment)
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

	status := statusFor(deployment)

	if status != deployapi.DeploymentStatusFailed {
		return fmt.Errorf("#%d is %s; only failed deployments can be retried", config.LatestVersion, status)
	}

	deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(deployapi.DeploymentStatusNew)
	_, err = c.client.UpdateDeployment(deployment)
	if err == nil {
		fmt.Fprintf(out, "retried #%d\n", config.LatestVersion)
	}
	return err
}

func statusFor(deployment *kapi.ReplicationController) deployapi.DeploymentStatus {
	return deployapi.DeploymentStatus(deployment.Annotations[deployapi.DeploymentStatusAnnotation])
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
