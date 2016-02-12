package controller

import (
	"fmt"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
)

// ConfigControllerFatalError represents a fatal error while generating a build.
// An operation that fails because of a fatal error should not be retried.
type ConfigControllerFatalError struct {
	// Reason the fatal error occurred
	Reason string
}

// Error returns the error string for this fatal error
func (e ConfigControllerFatalError) Error() string {
	return fmt.Sprintf("fatal error processing BuildConfig: %s", e.Reason)
}

// IsFatal returns true if err is a fatal error
func IsFatal(err error) bool {
	_, isFatal := err.(ConfigControllerFatalError)
	return isFatal
}

type BuildConfigController struct {
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
}

func (c *BuildConfigController) HandleBuildConfig(bc *buildapi.BuildConfig) error {
	glog.V(4).Infof("Handling BuildConfig %s/%s", bc.Namespace, bc.Name)

	hasChangeTrigger := false
	for _, trigger := range bc.Spec.Triggers {
		if trigger.Type == buildapi.ConfigChangeBuildTriggerType {
			hasChangeTrigger = true
			break
		}
	}

	if !hasChangeTrigger {
		return nil
	}

	if bc.Status.LastVersion > 0 {
		return nil
	}

	glog.V(4).Infof("Running build for BuildConfig %s/%s", bc.Namespace, bc.Name)
	// instantiate new build
	lastVersion := 0
	request := &buildapi.BuildRequest{
		ObjectMeta: kapi.ObjectMeta{
			Name:      bc.Name,
			Namespace: bc.Namespace,
		},
		LastVersion: &lastVersion,
	}
	if _, err := c.BuildConfigInstantiator.Instantiate(bc.Namespace, request); err != nil {
		var instantiateErr error
		if kerrors.IsConflict(err) {
			instantiateErr = fmt.Errorf("unable to instantiate Build for BuildConfig %s/%s due to a conflicting update: %v", bc.Namespace, bc.Name, err)
			utilruntime.HandleError(instantiateErr)
		} else if buildgenerator.IsFatal(err) {
			return &ConfigControllerFatalError{err.Error()}
		} else {
			instantiateErr = fmt.Errorf("error instantiating Build from BuildConfig %s/%s: %v", bc.Namespace, bc.Name, err)
			utilruntime.HandleError(instantiateErr)
		}
		return instantiateErr
	}
	return nil
}
