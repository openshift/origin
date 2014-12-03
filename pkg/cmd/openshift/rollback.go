package openshift

import (
	"flag"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/cobra"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"encoding/json"
	"io/ioutil"
)

type config struct {
	ClientConfig         *clientcmd.Config
	RollbackConfigName   string
}

func NewCommandRollback() *cobra.Command {
	flag.Set("v", "4")

	cfg := &config{
		ClientConfig: clientcmd.NewConfig(),
	}

	cmd := &cobra.Command{
		Use: "rollback",
		Run: func(c *cobra.Command, args []string) {
			if err := rollback(cfg, args); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flag := cmd.Flags()
	cfg.ClientConfig.Bind(flag)
	flag.StringVar(&cfg.RollbackConfigName, "f", "", "path/name of the rollback config file")

	return cmd
}

func rollback(cfg *config, args []string) error {
	glog.Info("------------------ Executing a rollback ----------------------")
	_, osClient, err := cfg.ClientConfig.Clients()

	if err != nil {
		return err
	}

	/*
		This is a mock up of the minimal viable product for rollbacks.  It is NOT a complete solution.

		A rollback is essentially a posting of an old deployment config on top of the current config.  However,
		the user will indicate which version of the deployment config they'd like to use by specifying a deployment
		from 'list deployments' or whatever the replication controller version evolves into.

		To demonstrate the mechanics the code below mocks what an actual api call would do.  In an actual solution
		I envision that it would be something like:

		1. user posts to rollback endpoint with .json containing config parameters or with GET params
		2. endpoint uses parameters to call DeploymentConfigRollbackGenerator.Generate(deploymentId) and receives
			a new deployment config back that is based on the old deployment
		3. endpoint performs validation
		4. endpoint calls DeploymentConfig.Update(newConfig)
	 */

	//1. user posts to rollback endpoint with .json containing config parameters or with GET params - currently bootstrapped on top of the DeploymentConfig
	glog.V(4).Infof("Reading rollback config")
	rollbackConfig, err := ReadRollbackFile(cfg.RollbackConfigName)
	if err != nil {
		return err
	}

	//2. endpoint uses parameters to call DeploymentConfigRollbackGenerator.Generate(deploymentId) and receives
	//   a new deployment config back that is based on the old deployment
	glog.V(4).Infof("Finding current deploy config named %s", rollbackConfig.ObjectMeta.Name)
	currentConfig, err := osClient.DeploymentConfigs("").Get(rollbackConfig.ObjectMeta.Name)
	if err != nil {
		return err
	}


	glog.V(4).Infof("Finding rollback deployment with name %s", rollbackConfig.Rollback.To)
	oldConfig, err := osClient.Deployments("").Get(rollbackConfig.Rollback.To)
	if err != nil {
		return err
	}

	currentConfig.Template.ControllerTemplate = oldConfig.ControllerTemplate

	//3. endpoint performs validation (todo)
	//4. endpoint calls DeploymentConfig.Update(newConfig)
	osClient.DeploymentConfigs("").Update(currentConfig)

	glog.Info("-------------------  Rollback Complete  ----------------------")
	return nil
}

func ReadRollbackFile(fileName string) (*deployapi.DeploymentConfig, error) {
	data, err := ioutil.ReadFile(fileName)

	if err != nil {
		return nil, err
	}

	rollbackConfig := &deployapi.DeploymentConfig{}
	err = json.Unmarshal(data, rollbackConfig)

	if err != nil {
		return nil, err
	}

	return rollbackConfig, nil
}
