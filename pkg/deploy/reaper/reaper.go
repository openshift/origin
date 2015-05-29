package reaper

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/deploy/api"
	deploy "github.com/openshift/origin/pkg/deploy/scaler"
	"github.com/openshift/origin/pkg/deploy/util"
)

const (
	shortInterval = time.Millisecond * 100
	interval      = time.Second * 3
	timeout       = time.Minute * 5
)

// ReaperFor returns the appropriate Reaper client depending on the provided
// kind of resource (Replication controllers, pods, services, and deploymentConfigs
// supported)
func ReaperFor(kind string, osc *client.Client, kc *kclient.Client) (kubectl.Reaper, error) {
	if kind != "DeploymentConfig" {
		return kubectl.ReaperFor(kind, kc)
	}
	return &DeploymentConfigReaper{osc: osc, kc: kc, pollInterval: interval, timeout: timeout}, nil
}

// DeploymentConfigReaper implements the Reaper interface for deploymentConfigs
type DeploymentConfigReaper struct {
	osc                   client.Interface
	kc                    kclient.Interface
	pollInterval, timeout time.Duration
}

// Stop scales a replication controller via its deployment configuration down to
// zero replicas, waits for all of them to get deleted and then deletes both the
// replication controller and its deployment configuration.
func (reaper *DeploymentConfigReaper) Stop(namespace, name string, gracePeriod *kapi.DeleteOptions) (string, error) {
	dc, err := reaper.osc.DeploymentConfigs(namespace).Get(name)
	if err != nil {
		return "", err
	}
	// Disable dc triggers while reaping
	dc.Triggers = []api.DeploymentTriggerPolicy{}
	dc, err = reaper.osc.DeploymentConfigs(namespace).Update(dc)
	if err != nil {
		return "", err
	}
	scaler, err := deploy.ScalerFor("DeploymentConfig", reaper.osc, reaper.kc)
	if err != nil {
		return "", err
	}
	retry := &kubectl.RetryParams{Interval: shortInterval, Timeout: reaper.timeout}
	waitForReplicas := &kubectl.RetryParams{reaper.pollInterval, reaper.timeout}
	if err = scaler.Scale(namespace, name, 0, nil, retry, waitForReplicas); err != nil {
		// The deploymentConfig may not have a replication controller to scale
		// so we shouldn't error out here
		glog.V(2).Info(err)
	}
	if err = reaper.kc.ReplicationControllers(namespace).Delete(util.LatestDeploymentNameForConfig(dc)); err != nil {
		// The deploymentConfig may not have a replication controller to delete
		// so we shouldn't error out here
		glog.V(2).Info(err)
	}
	if err = reaper.osc.DeploymentConfigs(namespace).Delete(name); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s stopped", name), nil
}
