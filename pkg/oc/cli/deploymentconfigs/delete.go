package deploymentconfigs

import (
	"time"

	"github.com/golang/glog"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	"github.com/openshift/origin/pkg/apps/util"
)

// NewDeploymentConfigReaper returns a new reaper for deploymentConfigs
func NewDeploymentConfigReaper(appsClient appsclient.Interface, kc kclientset.Interface) kubectl.Reaper {
	return &DeploymentConfigReaper{appsClient: appsClient, kc: kc, pollInterval: kubectl.Interval, timeout: kubectl.Timeout}
}

// DeploymentConfigReaper implements the Reaper interface for deploymentConfigs
type DeploymentConfigReaper struct {
	appsClient            appsclient.Interface
	kc                    kclientset.Interface
	pollInterval, timeout time.Duration
}

// pause marks the deployment configuration as paused to avoid triggering new
// deployments.
func (reaper *DeploymentConfigReaper) pause(namespace, name string) (*appsapi.DeploymentConfig, error) {
	patchBytes := []byte(`{"spec":{"paused":true,"replicas":0,"revisionHistoryLimit":0}}`)
	return reaper.appsClient.Apps().DeploymentConfigs(namespace).Patch(name, types.StrategicMergePatchType, patchBytes)
}

// Stop scales a replication controller via its deployment configuration down to
// zero replicas, waits for all of them to get deleted and then deletes both the
// replication controller and its deployment configuration.
func (reaper *DeploymentConfigReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	// Pause the deployment configuration to prevent the new deployments from
	// being triggered.
	config, err := reaper.pause(namespace, name)
	configNotFound := kerrors.IsNotFound(err)
	if err != nil && !configNotFound {
		return err
	}

	var (
		isPaused bool
		legacy   bool
	)
	// Determine if the deployment config controller noticed the pause.
	if !configNotFound {
		if err := wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
			dc, err := reaper.appsClient.Apps().DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			isPaused = dc.Spec.Paused
			return dc.Status.ObservedGeneration >= config.Generation, nil
		}); err != nil {
			return err
		}

		// If we failed to pause the deployment config, it means we are talking to
		// old API that does not support pausing. In that case, we delete the
		// deployment config to stay backward compatible.
		if !isPaused {
			if err := reaper.appsClient.Apps().DeploymentConfigs(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil {
				return err
			}
			// Setting this to true avoid deleting the config at the end.
			legacy = true
		}
	}

	// Clean up deployments related to the config. Even if the deployment
	// configuration has been deleted, we want to sweep the existing replication
	// controllers and clean them up.
	options := metav1.ListOptions{LabelSelector: util.ConfigSelector(name).String()}
	rcList, err := reaper.kc.Core().ReplicationControllers(namespace).List(options)
	if err != nil {
		return err
	}
	rcReaper, err := kubectl.ReaperFor(kapi.Kind("ReplicationController"), reaper.kc)
	if err != nil {
		return err
	}

	// If there is neither a config nor any deployments, nor any deployer pods, we can return NotFound.
	deployments := rcList.Items

	if configNotFound && len(deployments) == 0 {
		return kerrors.NewNotFound(kapi.Resource("deploymentconfig"), name)
	}

	for _, rc := range deployments {
		if err = rcReaper.Stop(rc.Namespace, rc.Name, timeout, gracePeriod); err != nil {
			// Better not error out here...
			glog.Infof("Cannot delete ReplicationController %s/%s for deployment config %s/%s: %v", rc.Namespace, rc.Name, namespace, name, err)
		}

		// Only remove deployer pods when the deployment was failed. For completed
		// deployment the pods should be already deleted.
		if !util.IsFailedDeployment(&rc) {
			continue
		}

		// Delete all deployer and hook pods
		podList, err := reaper.kc.Core().Pods(rc.Namespace).List(metav1.ListOptions{
			LabelSelector: util.DeployerPodSelector(rc.Name).String(),
		})
		if err != nil {
			return err
		}
		for _, pod := range podList.Items {
			err := reaper.kc.Core().Pods(pod.Namespace).Delete(pod.Name, gracePeriod)
			if err != nil {
				// Better not error out here...
				glog.Infof("Cannot delete lifecycle Pod %s/%s for deployment config %s/%s: %v", pod.Namespace, pod.Name, namespace, name, err)
			}
		}
	}

	// Nothing to delete or we already deleted the deployment config because we
	// failed to pause.
	if configNotFound || legacy {
		return nil
	}

	return reaper.appsClient.Apps().DeploymentConfigs(namespace).Delete(name, &metav1.DeleteOptions{})
}
