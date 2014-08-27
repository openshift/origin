package build

import (
	"fmt"
	"strings"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
)

// BuildJobStrategy represents a strategy for executing a build by
// creating a pod definition that will execute the build
type BuildJobStrategy interface {
	CreateBuildPod(build *api.Build, dockerImage string) *kubeapi.Pod
}

// BuildController watches build resources and manages their state
type BuildController struct {
	osClient        osclient.Interface
	kubeClient      kubeclient.Interface
	buildStrategies map[api.BuildType]BuildJobStrategy
	dockerRegistry  string
	timeout         int
}

// NewBuildController creates a new build controller
func NewBuildController(kc kubeclient.Interface,
	oc osclient.Interface,
	strategies map[api.BuildType]BuildJobStrategy,
	registry string,
	timeout int) *BuildController {

	glog.Infof("Creating build controller with dockerRegistry=%s, timeout=%d",
		registry, timeout)

	bc := &BuildController{
		kubeClient:      kc,
		osClient:        oc,
		buildStrategies: strategies,
		dockerRegistry:  registry,
		timeout:         timeout,
	}
	return bc

}

// Run begins watching and syncing build jobs onto the cluster.
func (bc *BuildController) Run(period time.Duration) {
	syncTime := time.Tick(period)
	go util.Forever(func() { bc.watchBuilds(syncTime) }, period)
}

// The main sync loop. Iterates over current builds and delegates syncing.
func (bc *BuildController) watchBuilds(syncTime <-chan time.Time) {
	for {
		select {
		case <-syncTime:
			builds, err := bc.osClient.ListBuilds(labels.Everything())
			if err != nil {
				glog.Errorf("Error listing builds: %v (%#v)", err, err)
				return
			}
			for _, build := range builds.Items {
				nextStatus, err := bc.synchronize(&build)
				if err != nil {
					glog.Errorf("Error synchronizing build ID %v: %#v", build.ID, err)
				}

				if nextStatus != build.Status {
					build.Status = nextStatus
					if _, err := bc.osClient.UpdateBuild(build); err != nil {
						glog.Errorf("Error updating build ID %v to status %v: %#v", build.ID, nextStatus, err)
					}
				}
			}
		}

	}
}

func hasTimeoutElapsed(build *api.Build, timeout int) bool {
	timestamp := build.CreationTimestamp
	elapsed := time.Since(timestamp.Time)
	return int(elapsed.Seconds()) > timeout
}

// Determine the next status of a build given its current state and the state
// of its associated pod.
// TODO: improve handling of illegal state transitions
func (bc *BuildController) synchronize(build *api.Build) (api.BuildStatus, error) {
	glog.Infof("Syncing build %s", build.ID)

	switch build.Status {
	case api.BuildNew:
		build.PodID = "build-" + string(build.Input.Type) + "-" + build.ID // TODO: better naming
		return api.BuildPending, nil
	case api.BuildPending:
		buildStrategy, ok := bc.buildStrategies[build.Input.Type]
		if !ok {
			return api.BuildError, fmt.Errorf("No build type for %s", build.Input.Type)
		}

		podSpec := buildStrategy.CreateBuildPod(build, bc.dockerRegistry)

		glog.Infof("Attempting to create pod: %#v", podSpec)
		_, err := bc.kubeClient.CreatePod(*podSpec)

		// TODO: strongly typed error checking
		if err != nil {
			if strings.Index(err.Error(), "already exists") != -1 {
				return build.Status, err // no transition, already handled by someone else
			}

			return api.BuildFailed, err
		}

		return api.BuildRunning, nil
	case api.BuildRunning:
		if timedOut := hasTimeoutElapsed(build, bc.timeout); timedOut {
			return api.BuildFailed, fmt.Errorf("Build timed out")
		}

		pod, err := bc.kubeClient.GetPod(build.PodID)
		if err != nil {
			return build.Status, fmt.Errorf("Error retrieving pod for build ID %v: %#v", build.ID, err)
		}

		// pod is still running
		if pod.CurrentState.Status != kubeapi.PodTerminated {
			return build.Status, nil
		}

		var nextStatus = api.BuildComplete

		// check the exit codes of all the containers in the pod
		for _, info := range pod.CurrentState.Info {
			if info.State.ExitCode != 0 {
				nextStatus = api.BuildFailed
			}
		}
		return nextStatus, nil
	case api.BuildComplete, api.BuildFailed, api.BuildError:
		return build.Status, nil
	default:
		return api.BuildError, fmt.Errorf("Invalid build status: %s", build.Status)
	}
}
