package deployer

import (
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	appsv1 "github.com/openshift/api/apps/v1"
	imageclientv1 "github.com/openshift/client-go/image/clientset/versioned"
	"github.com/openshift/origin/pkg/apps/strategy"
	"github.com/openshift/origin/pkg/apps/strategy/recreate"
	"github.com/openshift/origin/pkg/apps/strategy/rolling"
	appsutil "github.com/openshift/origin/pkg/apps/util"
	"github.com/openshift/origin/pkg/cmd/util"
	cmdversion "github.com/openshift/origin/pkg/cmd/version"
	"github.com/openshift/origin/pkg/version"
)

var (
	deployerLong = templates.LongDesc(`
		Perform a deployment

		This command launches a deployment as described by a deployment configuration. It accepts the name
		of a replication controller created by a deployment and runs that deployment to completion. You can
		use the --until flag to run the deployment until you reach the specified condition.

		Available conditions:

		* "start": after old deployments are scaled to zero
		* "pre": after the pre hook completes (even if no hook specified)
		* "mid": after the mid hook completes (even if no hook specified)
		* A percentage of the deployment, based on which strategy is in use
		  * "0%"   Recreate after the previous deployment is scaled to zero
		  * "N%"   Recreate after the acceptance check if this is not the first deployment
		  * "0%"   Rolling  before the rolling deployment is started, equivalent to "pre"
		  * "N%"   Rolling  the percentage of pods in the target deployment that are ready
		  * "100%" All      after the deployment is at full scale, but before the post hook runs

		Unrecognized conditions will be ignored and the deployment will run to completion. You can run this
		command multiple times when --until is specified - hooks will only be executed once.`)
)

type config struct {
	Out, ErrOut io.Writer

	rcName    string
	Namespace string

	Until string
}

// NewCommandDeployer provides a CLI handler for deploy.
func NewCommandDeployer(name string) *cobra.Command {
	cfg := &config{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [--until=CONDITION]", name),
		Short: "Run the deployer",
		Long:  deployerLong,
		Run: func(c *cobra.Command, args []string) {
			cfg.Out = os.Stdout
			cfg.ErrOut = c.OutOrStderr()
			err := cfg.RunDeployer()
			if strategy.IsConditionReached(err) {
				fmt.Fprintf(os.Stdout, "--> %s\n", err.Error())
				return
			}
			kcmdutil.CheckErr(err)
		},
	}

	cmd.AddCommand(cmdversion.NewCmdVersion(name, version.Get(), os.Stdout))

	flag := cmd.Flags()
	flag.StringVar(&cfg.rcName, "deployment", util.Env("OPENSHIFT_DEPLOYMENT_NAME", ""), "The deployment name to start")
	flag.StringVar(&cfg.Namespace, "namespace", util.Env("OPENSHIFT_DEPLOYMENT_NAMESPACE", ""), "The deployment namespace")
	flag.StringVar(&cfg.Until, "until", "", "Exit the deployment when this condition is met. See help for more details")

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
	openshiftImageClient, err := imageclientv1.NewForConfig(kcfg)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(kcfg)
	if err != nil {
		return err
	}

	deployer := NewDeployer(kubeClient, openshiftImageClient, cfg.Out, cfg.ErrOut, cfg.Until)
	return deployer.Deploy(cfg.Namespace, cfg.rcName)
}

// NewDeployer makes a new Deployer from a kube client.
func NewDeployer(kubeClient kubernetes.Interface, images imageclientv1.Interface, out, errOut io.Writer,
	until string) *Deployer {
	return &Deployer{
		out:    out,
		errOut: errOut,
		until:  until,
		getDeployment: func(namespace, name string) (*corev1.ReplicationController, error) {
			return kubeClient.CoreV1().ReplicationControllers(namespace).Get(name, metav1.GetOptions{})
		},
		getDeployments: func(namespace, configName string) (*corev1.ReplicationControllerList, error) {
			return kubeClient.CoreV1().ReplicationControllers(namespace).List(metav1.ListOptions{LabelSelector: appsutil.ConfigSelector(configName).
				String()})
		},
		scaler: appsutil.NewReplicationControllerScaler(kubeClient),
		strategyFor: func(config *appsv1.DeploymentConfig) (strategy.DeploymentStrategy, error) {
			switch config.Spec.Strategy.Type {
			case appsv1.DeploymentStrategyTypeRecreate:
				return recreate.NewRecreateDeploymentStrategy(kubeClient, images.ImageV1(),
					&kv1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")}, out, errOut, until), nil
			case appsv1.DeploymentStrategyTypeRolling:
				recreateDeploymentStrategy := recreate.NewRecreateDeploymentStrategy(kubeClient, images.ImageV1(),
					&kv1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")}, out, errOut, until)
				return rolling.NewRollingDeploymentStrategy(config.Namespace, kubeClient, images.ImageV1(),
					recreateDeploymentStrategy, out, errOut, until), nil
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
	// until is a condition to run until
	until string
	// strategyFor returns a DeploymentStrategy for config.
	strategyFor func(config *appsv1.DeploymentConfig) (strategy.DeploymentStrategy, error)
	// getDeployment finds the named deployment.
	getDeployment func(namespace, name string) (*corev1.ReplicationController, error)
	// getDeployments finds all deployments associated with a config.
	getDeployments func(namespace, configName string) (*corev1.ReplicationControllerList, error)
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
	// TODO: Remove this once we are sure there are no internal versions of configs serialized in DC
	config, err := appsutil.DecodeDeploymentConfig(to)
	if err != nil {
		return fmt.Errorf("couldn't decode deployment config from deployment %s: %v", to.Name, err)
	}

	// Get a strategy for the deployment.
	s, err := d.strategyFor(config)
	if err != nil {
		return err
	}

	// New deployments must have a desired replica count.
	desiredReplicas, hasDesired := appsutil.DeploymentDesiredReplicas(to)
	if !hasDesired {
		return fmt.Errorf("deployment %s has already run to completion", to.Name)
	}

	// Find all deployments for the config.
	unsortedDeployments, err := d.getDeployments(namespace, config.Name)
	if err != nil {
		return fmt.Errorf("couldn't get controllers in namespace %s: %v", namespace, err)
	}
	deployments := make([]*corev1.ReplicationController, 0, len(unsortedDeployments.Items))
	for i := range unsortedDeployments.Items {
		deployments = append(deployments, &unsortedDeployments.Items[i])
	}

	// Sort all the deployments by version.
	sort.Sort(appsutil.ByLatestVersionDesc(deployments))

	// Find any last completed deployment.
	var from *corev1.ReplicationController
	for _, candidate := range deployments {
		if candidate.Name == to.Name {
			continue
		}
		if appsutil.IsCompleteDeployment(candidate) {
			from = candidate
			break
		}
	}

	if appsutil.DeploymentVersionFor(to) < appsutil.DeploymentVersionFor(from) {
		return fmt.Errorf("deployment %s is older than %s", to.Name, from.Name)
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
		if candidate.Spec.Replicas == nil || *candidate.Spec.Replicas == 0 {
			continue
		}
		// Scale the deployment down to zero.
		retryWaitParams := kubectl.NewRetryParams(1*time.Second, 120*time.Second)
		if err := d.scaler.Scale(candidate.Namespace, candidate.Name, uint(0), &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}, retryWaitParams, retryWaitParams, kapi.Resource("replicationcontrollers")); err != nil {
			fmt.Fprintf(d.errOut, "error: Couldn't scale down prior deployment %s: %v\n", appsutil.LabelForDeployment(candidate), err)
		} else {
			fmt.Fprintf(d.out, "--> Scaled older deployment %s down\n", candidate.Name)
		}
	}

	if d.until == "start" {
		return strategy.NewConditionReachedErr("Ready to start deployment")
	}

	// Perform the deployment.
	if err := s.Deploy(from, to, int(desiredReplicas)); err != nil {
		return err
	}
	fmt.Fprintln(d.out, "--> Success")
	return nil
}
