package reaper

import (
	"fmt"
	"strings"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigReaper makes a new DeploymentConfigReaper.
func NewDeploymentConfigReaper(oc client.Interface, kc kclient.Interface, interval, timeout time.Duration) *DeploymentConfigReaper {
	return &DeploymentConfigReaper{
		oc:           oc,
		kc:           kc,
		pollInterval: interval,
		timeout:      timeout,
	}
}

// DeploymentConfigReaper implements the Reaper interface for deploymentConfigs
type DeploymentConfigReaper struct {
	oc                    client.Interface
	kc                    kclient.Interface
	pollInterval, timeout time.Duration
}

// Stop scales a replication controller via its deployment configuration down to
// zero replicas, waits for all of them to get deleted and then deletes both the
// replication controller and its deployment configuration.
func (reaper *DeploymentConfigReaper) Stop(namespace, name string, gracePeriod *kapi.DeleteOptions) (string, error) {
	// If the config is already deleted, it may still have associated
	// deployments which didn't get cleaned up during prior calls to Stop. If
	// the config can't be found, still make an attempt to clean up the
	// deployments.
	//
	// It's important to delete the config first to avoid an undesirable side
	// effect which can cause the deployment to be re-triggered upon the
	// config's deletion. See https://github.com/openshift/origin/issues/2721
	// for more details.
	err := reaper.oc.DeploymentConfigs(namespace).Delete(name)
	configNotFound := kerrors.IsNotFound(err)
	if err != nil && !configNotFound {
		return "", err
	}

	// Clean up deployments related to the config.
	rcList, err := reaper.kc.ReplicationControllers(namespace).List(util.ConfigSelector(name))
	if err != nil {
		return "", err
	}
	rcReaper, err := kubectl.ReaperFor("ReplicationController", reaper.kc)
	if err != nil {
		return "", err
	}

	// If there is neither a config nor any deployments, we can return NotFound.
	deployments := rcList.Items
	if configNotFound && len(deployments) == 0 {
		return "", kerrors.NewNotFound("DeploymentConfig", name)
	}
	for _, rc := range deployments {
		if _, err = rcReaper.Stop(rc.Namespace, rc.Name, gracePeriod); err != nil {
			// Better not error out here...
			glog.Infof("Cannot delete ReplicationController %s/%s: %v", rc.Namespace, rc.Name, err)
		}
	}

	return fmt.Sprintf("%s stopped", name), nil
}

// NewDeploymentReaper creates a new DeploymentReaper.
func NewDeploymentReaper(controllerReaper kubectl.Reaper, podReaper kubectl.Reaper, kc kclient.Interface) *DeploymentReaper {
	return &DeploymentReaper{
		controllerReaper: controllerReaper,
		podReaper:        podReaper,
		deployerPodsFor: func(namespace, name string) (*kapi.PodList, error) {
			return kc.Pods(namespace).List(util.DeployerPodSelector(name), fields.Everything())
		},
	}
}

// DeploymentReaper is a Reaper for deployments. It reaps all deployer pods
// for the deployment using a Pod reaper, and then reaps the deployment itself
// using a ReplicationController reaper.
type DeploymentReaper struct {
	// controllerReaper is used to reap the deployment itself.
	controllerReaper kubectl.Reaper
	// podReaper is used to reap deployer pods.
	podReaper kubectl.Reaper
	// deployerPodsFor returns all deployer pods associated with a deployment.
	deployerPodsFor func(namespace, name string) (*kapi.PodList, error)
}

// Stop implements Reaper.
func (r *DeploymentReaper) Stop(namespace, name string, gracePeriod *kapi.DeleteOptions) (string, error) {
	// Find all deployer pods for the deployment.
	pods, err := r.deployerPodsFor(namespace, name)
	if err != nil {
		return "", err
	}

	var output []string

	// Stop any deployer pods. Log errors.
	for _, pod := range pods.Items {
		result, err := r.podReaper.Stop(pod.Namespace, pod.Name, gracePeriod)
		if err != nil {
			glog.Infof("Couldn't delete deployer pod %q: %v", pod.Name, err)
		} else {
			output = append(output, result)
		}
	}

	// Stop the deployment.
	result, err := r.controllerReaper.Stop(namespace, name, gracePeriod)
	if err != nil {
		return "", err
	}
	output = append(output, result)

	return strings.Join(output, "\n"), nil
}
