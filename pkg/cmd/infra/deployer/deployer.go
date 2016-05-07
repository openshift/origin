package deployer

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
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
	Out, ErrOut io.Writer

	rcName    string
	Namespace string
}

// NewCommandDeployer provides a CLI handler for deploy.
func NewCommandDeployer(name string) *cobra.Command {
	cfg := &config{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Run the deployer",
		Long:  deployerLong,
		Run: func(c *cobra.Command, args []string) {
			cfg.Out = os.Stdout
			cfg.ErrOut = c.Out()
			err := cfg.RunDeployer()
			kcmdutil.CheckErr(err)
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name, false))

	flag := cmd.Flags()
	flag.StringVar(&cfg.rcName, "deployment", util.Env("OPENSHIFT_DEPLOYMENT_NAME", ""), "The deployment name to start")
	flag.StringVar(&cfg.Namespace, "namespace", util.Env("OPENSHIFT_DEPLOYMENT_NAMESPACE", ""), "The deployment namespace")

	return cmd
}

func (cfg *config) RunDeployer() error {
	if len(cfg.rcName) == 0 {
		return fmt.Errorf("--deployment or OPENSHIFT_DEPLOYMENT_NAME is required")
	}
	if len(cfg.Namespace) == 0 {
		return fmt.Errorf("--namespace or OPENSHIFT_DEPLOYMENT_NAMESPACE is required")
	}

	kcfg, err := restclient.InClusterConfig()
	if err != nil {
		return err
	}
	kc, err := kclient.New(kcfg)
	if err != nil {
		return err
	}
	oc, err := client.New(kcfg)
	if err != nil {
		return err
	}

	deployer := NewDeployer(kc, oc, cfg.Out, cfg.ErrOut)
	return deployer.Deploy(cfg.Namespace, cfg.rcName)
}

// NewDeployer makes a new Deployer from a kube client.
func NewDeployer(client kclient.Interface, oclient client.Interface, out, errOut io.Writer) *Deployer {
	scaler, _ := kubectl.ScalerFor(kapi.Kind("ReplicationController"), client)
	return &Deployer{
		out:    out,
		errOut: errOut,
		getDeployment: func(namespace, name string) (*kapi.ReplicationController, error) {
			return client.ReplicationControllers(namespace).Get(name)
		},
		getDeployments: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
			return client.ReplicationControllers(namespace).List(kapi.ListOptions{LabelSelector: deployutil.ConfigSelector(configName)})
		},
		scaler: scaler,
		strategyFor: func(config *deployapi.DeploymentConfig) (strategy.DeploymentStrategy, error) {
			switch config.Spec.Strategy.Type {
			case deployapi.DeploymentStrategyTypeRecreate:
				return recreate.NewRecreateDeploymentStrategy(client, oclient, kapi.Codecs.UniversalDecoder(), out, errOut), nil
			case deployapi.DeploymentStrategyTypeRolling:
				recreate := recreate.NewRecreateDeploymentStrategy(client, oclient, kapi.Codecs.UniversalDecoder(), out, errOut)
				return rolling.NewRollingDeploymentStrategy(config.Namespace, client, oclient, kapi.Codecs.UniversalDecoder(), recreate, out, errOut), nil
			default:
				return nil, fmt.Errorf("unsupported strategy type: %s", config.Spec.Strategy.Type)
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
	// out and errOut control display when deploy is invoked
	out, errOut io.Writer
	// strategyFor returns a DeploymentStrategy for config.
	strategyFor func(config *deployapi.DeploymentConfig) (strategy.DeploymentStrategy, error)
	// getDeployment finds the named deployment.
	getDeployment func(namespace, name string) (*kapi.ReplicationController, error)
	// getDeployments finds all deployments associated with a config.
	getDeployments func(namespace, configName string) (*kapi.ReplicationControllerList, error)
	// scaler is used to scale replication controllers.
	scaler kubectl.Scaler
}

// Deploy starts the deployment process for rcName.
func (d *Deployer) Deploy(namespace, rcName string) error {
	// Look up the new deployment.
	to, err := d.getDeployment(namespace, rcName)
	if err != nil {
		return fmt.Errorf("couldn't get deployment %s: %v", rcName, err)
	}

	// Decode the config from the deployment.
	config, err := deployutil.DecodeDeploymentConfig(to, kapi.Codecs.UniversalDecoder())
	if err != nil {
		return fmt.Errorf("couldn't decode deployment config from deployment %s: %v", to.Name, err)
	}

	// Get a strategy for the deployment.
	strategy, err := d.strategyFor(config)
	if err != nil {
		return err
	}

	// New deployments must have a desired replica count.
	desiredReplicas, hasDesired := deployutil.DeploymentDesiredReplicas(to)
	if !hasDesired {
		return fmt.Errorf("deployment %s has already run to completion", to.Name)
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
			fmt.Fprintf(d.errOut, "error: Couldn't scale down prior deployment %s: %v\n", deployutil.LabelForDeployment(&candidate), err)
		} else {
			fmt.Fprintf(d.out, "--> Scaled older deployment %s down\n", candidate.Name)
		}
	}

	// Perform the deployment.
	if err := strategy.Deploy(from, to, desiredReplicas); err != nil {
		return err
	}
	fmt.Fprintf(d.out, "--> Success\n")
	return nil
}
