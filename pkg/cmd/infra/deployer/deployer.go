package deployer

import (
	"fmt"
	"strconv"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	strategy "github.com/openshift/origin/pkg/deploy/strategy/recreate"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
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

type replicationControllerGetter interface {
	Get(namespace, name string) (*kapi.ReplicationController, error)
	List(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error)
}

// NewCommandDeployer provides a CLI handler for deploy.
func NewCommandDeployer(name string) *cobra.Command {
	cfg := &config{
		Config: clientcmd.NewConfig(),
	}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s%s", name, clientcmd.ConfigSyntax),
		Short: "Run the OpenShift deployer",
		Long:  longCommandDesc,
		Run: func(c *cobra.Command, args []string) {
			kClient, _, err := cfg.Config.Clients()
			if err != nil {
				glog.Fatal(err)
			}

			if len(cfg.DeploymentName) == 0 {
				glog.Fatal("deployment is required")
			}

			if len(cfg.Namespace) == 0 {
				glog.Fatal("namespace is required")
			}

			if err = deploy(kClient, cfg.Namespace, cfg.DeploymentName); err != nil {
				glog.Fatal(err)
			}
		},
	}

	cmd.SetUsageTemplate(templates.MainUsageTemplate())
	cmd.SetHelpTemplate(templates.MainHelpTemplate())

	flag := cmd.Flags()
	cfg.Config.Bind(flag)
	flag.StringVar(&cfg.DeploymentName, "deployment", util.Env("OPENSHIFT_DEPLOYMENT_NAME", ""), "The deployment name to start")
	flag.StringVar(&cfg.Namespace, "namespace", util.Env("OPENSHIFT_DEPLOYMENT_NAMESPACE", ""), "The deployment namespace")

	return cmd
}

// deploy executes a deployment strategy.
func deploy(kClient kclient.Interface, namespace, deploymentName string) error {
	newDeployment, oldDeployments, err := getDeployerContext(&realReplicationControllerGetter{kClient}, namespace, deploymentName)

	if err != nil {
		return err
	}

	// TODO: Choose a strategy based on some input
	strategy := strategy.NewRecreateDeploymentStrategy(kClient, latest.Codec)
	return strategy.Deploy(newDeployment, oldDeployments)
}

// getDeployerContext finds the target deployment and any deployments it considers to be prior to the
// target deployment. Only deployments whose LatestVersion is less than the target deployment are
// considered to be prior.
func getDeployerContext(controllerGetter replicationControllerGetter, namespace, deploymentName string) (*kapi.ReplicationController, []kapi.ObjectReference, error) {
	var err error
	var newDeployment *kapi.ReplicationController
	var newConfig *deployapi.DeploymentConfig

	// Look up the new deployment and its associated config.
	if newDeployment, err = controllerGetter.Get(namespace, deploymentName); err != nil {
		return nil, nil, err
	}

	if newConfig, err = deployutil.DecodeDeploymentConfig(newDeployment, latest.Codec); err != nil {
		return nil, nil, err
	}

	glog.Infof("Found new deployment %s for config %s with latestVersion %d", newDeployment.Name, newConfig.Name, newConfig.LatestVersion)

	// Collect all deployments that predate the new one by comparing all old ReplicationControllers with
	// encoded DeploymentConfigs to the new one by LatestVersion. Treat a failure to interpret a given
	// old deployment as a fatal error to prevent overlapping deployments.
	var allControllers *kapi.ReplicationControllerList
	oldDeployments := []kapi.ObjectReference{}

	if allControllers, err = controllerGetter.List(newDeployment.Namespace, labels.Everything()); err != nil {
		return nil, nil, fmt.Errorf("Unable to get list replication controllers in deployment namespace %s: %v", newDeployment.Namespace, err)
	}

	glog.Infof("Inspecting %d potential prior deployments", len(allControllers.Items))
	for _, controller := range allControllers.Items {
		if configName, hasConfigName := controller.Annotations[deployapi.DeploymentConfigAnnotation]; !hasConfigName {
			glog.Infof("Disregarding replicationController %s (not a deployment)", controller.Name)
			continue
		} else if configName != newConfig.Name {
			glog.Infof("Disregarding deployment %s (doesn't match target deploymentConfig %s)", controller.Name, configName)
			continue
		}

		var oldVersion int
		if oldVersion, err = strconv.Atoi(controller.Annotations[deployapi.DeploymentVersionAnnotation]); err != nil {
			return nil, nil, fmt.Errorf("Couldn't determine version of deployment %s: %v", controller.Name, err)
		}

		if oldVersion < newConfig.LatestVersion {
			glog.Infof("Marking deployment %s as a prior deployment", controller.Name)
			oldDeployments = append(oldDeployments, kapi.ObjectReference{
				Namespace: controller.Namespace,
				Name:      controller.Name,
			})
		} else {
			glog.Infof("Disregarding deployment %s (same as or newer than target)", controller.Name)
		}
	}

	return newDeployment, oldDeployments, nil
}

type realReplicationControllerGetter struct {
	kClient kclient.Interface
}

func (r *realReplicationControllerGetter) Get(namespace, name string) (*kapi.ReplicationController, error) {
	return r.kClient.ReplicationControllers(namespace).Get(name)
}

func (r *realReplicationControllerGetter) List(namespace string, selector labels.Selector) (*kapi.ReplicationControllerList, error) {
	return r.kClient.ReplicationControllers(namespace).List(selector)
}
