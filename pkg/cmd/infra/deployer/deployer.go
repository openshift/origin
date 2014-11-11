package deployer

import (
	"errors"
	"fmt"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/deploy/deployer/customimage"
)

const longCommandDesc = `
Perform a Deployment

This command makes calls to OpenShift to perform a deployment as described by a deployment config.
`

type config struct {
	Config         *clientcmd.Config
	DeploymentName string
}

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
	flag.StringVar(&cfg.DeploymentName, "deployment", util.Env("KUBERNETES_DEPLOYMENT_ID", ""), "The deployment name to start")

	return cmd
}

// deploy starts the deployer
func deploy(cfg *config) error {
	kClient, osClient, err := cfg.Config.Clients()
	if err != nil {
		return err
	}
	if len(cfg.DeploymentName) == 0 {
		return errors.New("No deployment name was specified. Expected KUBERNETES_DEPLOYMENT_ID variable.")
	}

	d := customimage.CustomImageDeployer{kClient, osClient}
	return d.Deploy(cfg.DeploymentName)
}
