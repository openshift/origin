package deployer

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	strategy "github.com/openshift/origin/pkg/deploy/strategy/recreate"
)

const longCommandDesc = `
Perform a Deployment

This command makes calls to OpenShift to perform a deployment as described by a deployment config.
`

type config struct {
	Config         *clientcmd.Config
	DeploymentName string
	Namespace      string
}

// NewCommandDeployer provides a CLI handler for deploy
func NewCommandDeployer(name string) *cobra.Command {
	cfg := &config{
		Config: clientcmd.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Run the OpenShift deployer",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			if err := deploy(cfg); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()
	cfg.Config.Bind(flag)
	flag.StringVar(&cfg.DeploymentName, "deployment", util.Env("OPENSHIFT_DEPLOYMENT_NAME", ""), "The deployment name to start")
	flag.StringVar(&cfg.Namespace, "namespace", util.Env("OPENSHIFT_DEPLOYMENT_NAMESPACE", ""), "The deployment namespace")

	return cmd
}

// deploy starts the deployer
func deploy(cfg *config) error {
	kClient, _, err := cfg.Config.Clients()
	if err != nil {
		return err
	}
	if len(cfg.DeploymentName) == 0 {
		return errors.New("No deployment name was specified.")
	}

	var deployment *kapi.ReplicationController
	if deployment, err = kClient.ReplicationControllers(cfg.Namespace).Get(cfg.DeploymentName); err != nil {
		return err
	}

	// TODO: Choose a strategy based on some input
	strategy := &strategy.DeploymentStrategy{
		ReplicationController: strategy.RealReplicationController{KubeClient: kClient},
		Codec: latest.Codec,
	}
	return strategy.Deploy(deployment)
}
