package reaper

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/deploy/util"
)

// ReaperFor returns the appropriate Reaper client depending on the provided
// kind of resource (Replication controllers, pods, services, and deploymentConfigs
// supported)
func ReaperFor(kind string, osc *client.Client, kc *kclient.Client) (kubectl.Reaper, error) {
	if kind != "DeploymentConfig" {
		return kubectl.ReaperFor(kind, kc)
	}
	return &DeploymentConfigReaper{osc: osc, kc: kc, pollInterval: kubectl.Interval, timeout: kubectl.Timeout}, nil
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
	rcList, err := reaper.kc.ReplicationControllers(namespace).List(labels.Everything())
	if err != nil {
		return "", err
	}
	rcReaper, err := kubectl.ReaperFor("ReplicationController", reaper.kc)
	if err != nil {
		return "", err
	}
	// Remove all the deployments of the configuration
	for _, rc := range util.ConfigSelector(dc, rcList.Items) {
		if _, err = rcReaper.Stop(rc.Namespace, rc.Name, gracePeriod); err != nil {
			// Better not error out here...
			glog.Infof("Cannot delete ReplicationController %s/%s: %v", rc.Namespace, rc.Name, err)
		}
	}
	if err = reaper.osc.DeploymentConfigs(namespace).Delete(name); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s stopped", name), nil
}
