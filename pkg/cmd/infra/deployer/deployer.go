package deployer

import (
	"fmt"
	"sort"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/strategy"
	"github.com/openshift/origin/pkg/deploy/strategy/recreate"
	"github.com/openshift/origin/pkg/deploy/strategy/rolling"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	"github.com/openshift/origin/pkg/version"
)

const (
	deployerLong = `
Perform a deployment

This command launches a deployment as described by a deployment configuration.`
)

type config struct {
	Config         *clientcmd.Config
	DeploymentName string
	Namespace      string
}

// NewCommandDeployer provides a CLI handler for deploy.
func NewCommandDeployer(name string) *cobra.Command {
	cfg := &config{
		Config: clientcmd.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Run the deployer",
		Long:  deployerLong,
		Run: func(c *cobra.Command, args []string) {
			_, kClient, err := cfg.Config.Clients()
			if err != nil {
				glog.Fatal(err)
			}

			if len(cfg.DeploymentName) == 0 {
				glog.Fatal("deployment is required")
			}

			if len(cfg.Namespace) == 0 {
				glog.Fatal("namespace is required")
			}

			deployer := NewDeployer(kClient)
			if err = deployer.Deploy(cfg.Namespace, cfg.DeploymentName); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name, false))

	flag := cmd.Flags()
	cfg.Config.Bind(flag)
	flag.StringVar(&cfg.DeploymentName, "deployment", util.Env("OPENSHIFT_DEPLOYMENT_NAME", ""), "The deployment name to start")
	flag.StringVar(&cfg.Namespace, "namespace", util.Env("OPENSHIFT_DEPLOYMENT_NAMESPACE", ""), "The deployment namespace")

	return cmd
}

// NewDeployer makes a new Deployer from a kube client.
func NewDeployer(client kclient.Interface) *Deployer {
	scaler, _ := kubectl.ScalerFor("ReplicationController", kubectl.NewScalerClient(client))
	return &Deployer{
		getDeployment: func(namespace, name string) (*kapi.ReplicationController, error) {
			return client.ReplicationControllers(namespace).Get(name)
		},
		getDeployments: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
			return client.ReplicationControllers(namespace).List(deployutil.ConfigSelector(configName))
		},
		scaler: scaler,
		strategyFor: func(config *deployapi.DeploymentConfig) (strategy.DeploymentStrategy, error) {
			switch config.Template.Strategy.Type {
			case deployapi.DeploymentStrategyTypeRecreate:
				return recreate.NewRecreateDeploymentStrategy(client, latest.Codec), nil
			case deployapi.DeploymentStrategyTypeRolling:
				recreate := recreate.NewRecreateDeploymentStrategy(client, latest.Codec)
				return rolling.NewRollingDeploymentStrategy(config.Namespace, client, latest.Codec, recreate), nil
			default:
				return nil, fmt.Errorf("unsupported strategy type: %s", config.Template.Strategy.Type)
			}
		},
	}
}

// Deployer prepares and executes the deployment process. It will:
//
// 1. Validate the deployment has a desired replica count and strategy.
// 2. Find the last completed deployment.
// 3. Scale down to 0 any old deployments which aren't the new deployment or
// the last complete deployment.
// 4. Pass the last completed deployment and the new deployment to a strategy
// to perform the deployment.
type Deployer struct {
	// strategyFor returns a DeploymentStrategy for config.
	strategyFor func(config *deployapi.DeploymentConfig) (strategy.DeploymentStrategy, error)
	// getDeployment finds the named deployment.
	getDeployment func(namespace, name string) (*kapi.ReplicationController, error)
	// getDeployments finds all deployments associated with a config.
	getDeployments func(namespace, configName string) (*kapi.ReplicationControllerList, error)
	// scaler is used to scale replication controllers.
	scaler kubectl.Scaler
}

// Deploy starts the deployment process for deploymentName.
func (d *Deployer) Deploy(namespace, deploymentName string) error {
	// Look up the new deployment.
	to, err := d.getDeployment(namespace, deploymentName)
	if err != nil {
		return fmt.Errorf("couldn't get deployment %s/%s: %v", namespace, deploymentName, err)
	}

	// Decode the config from the deployment.
	config, err := deployutil.DecodeDeploymentConfig(to, latest.Codec)
	if err != nil {
		return fmt.Errorf("couldn't decode deployment config from deployment %s/%s: %v", to.Namespace, to.Name, err)
	}

	// Get a strategy for the deployment.
	strategy, err := d.strategyFor(config)
	if err != nil {
		return err
	}

	// New deployments must have a desired replica count.
	desiredReplicas, hasDesired := deployutil.DeploymentDesiredReplicas(to)
	if !hasDesired {
		return fmt.Errorf("deployment %s has no desired replica count", deployutil.LabelForDeployment(to))
	}

	// Find all deployments for the config.
	unsortedDeployments, err := d.getDeployments(namespace, config.Name)
	if err != nil {
		return fmt.Errorf("couldn't get controllers in namespace %s: %v", namespace, err)
	}
	deployments := unsortedDeployments.Items

	// Sort all the deployments by version.
	sort.Sort(deployutil.ByLatestVersionDesc(deployments))

	// Find any last completed deployment.
	var from *kapi.ReplicationController
	for _, candidate := range deployments {
		if candidate.Name == to.Name {
			continue
		}
		if deployutil.DeploymentStatusFor(&candidate) == deployapi.DeploymentStatusComplete {
			from = &candidate
			break
		}
	}

	// Scale down any deployments which aren't the new or last deployment.
	for _, candidate := range deployments {
		// Skip the from/to deployments.
		if candidate.Name == to.Name {
			continue
		}
		if from != nil && candidate.Name == from.Name {
			continue
		}
		// Skip the deployment if it's already scaled down.
		if candidate.Spec.Replicas == 0 {
			continue
		}
		// Scale the deployment down to zero.
		retryWaitParams := kubectl.NewRetryParams(1*time.Second, 120*time.Second)
		if err := d.scaler.Scale(candidate.Namespace, candidate.Name, uint(0), &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}, retryWaitParams, retryWaitParams); err != nil {
			glog.Errorf("Couldn't scale down prior deployment %s: %v", deployutil.LabelForDeployment(&candidate), err)
		} else {
			glog.Infof("Scaled down prior deployment %s", deployutil.LabelForDeployment(&candidate))
		}
	}

	// Perform the deployment.
	if from == nil {
		glog.Infof("Deploying %s for the first time (replicas: %d)", deployutil.LabelForDeployment(to), desiredReplicas)
	} else {
		glog.Infof("Deploying from %s to %s (replicas: %d)", deployutil.LabelForDeployment(from), deployutil.LabelForDeployment(to), desiredReplicas)
	}
	return strategy.Deploy(from, to, desiredReplicas)
}
