package reaper

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/kubectl"

	"github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
)

// NewBuildConfigReaper returns a new reaper for buildConfigs
func NewBuildConfigReaper(oc *client.Client) kubectl.Reaper {
	return &BuildConfigReaper{oc: oc, pollInterval: kubectl.Interval, timeout: kubectl.Timeout}
}

// BuildConfigReaper implements the Reaper interface for buildConfigs
type BuildConfigReaper struct {
	oc                    client.Interface
	pollInterval, timeout time.Duration
}

// Stop deletes the build configuration and all of the associated builds.
func (reaper *BuildConfigReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *kapi.DeleteOptions) (string, error) {
	// If the config is already deleted, it may still have associated
	// builds which didn't get cleaned up during prior calls to Stop. If
	// the config can't be found, still make an attempt to clean up the
	// builds.
	//
	// We delete the config first to ensure no further builds will be created.
	err := reaper.oc.BuildConfigs(namespace).Delete(name)
	configNotFound := kerrors.IsNotFound(err)
	if err != nil && !configNotFound {
		return "", err
	}

	// Clean up builds related to the config.
	buildList, err := reaper.oc.Builds(namespace).List(util.ConfigSelector(name), nil)
	if err != nil {
		return "", err
	}

	// If there is neither a config nor any builds, we can return NotFound.
	builds := buildList.Items
	if configNotFound && len(builds) == 0 {
		return "", kerrors.NewNotFound("BuildConfig", name)
	}
	for _, build := range builds {
		if err = reaper.oc.Builds(namespace).Delete(build.Name); err != nil {
			glog.Infof("Cannot delete Build %s/%s: %v", build.Namespace, build.Name, err)
		}
	}

	return fmt.Sprintf("%s stopped", name), nil
}
