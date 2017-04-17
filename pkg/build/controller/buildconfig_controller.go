package controller

import (
	"fmt"

	"github.com/golang/glog"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/controller/common"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
)

// ConfigControllerFatalError represents a fatal error while generating a build.
// An operation that fails because of a fatal error should not be retried.
type ConfigControllerFatalError struct {
	// Reason the fatal error occurred
	Reason string
}

// Error returns the error string for this fatal error
func (e *ConfigControllerFatalError) Error() string {
	return fmt.Sprintf("fatal error processing BuildConfig: %s", e.Reason)
}

// IsFatal returns true if err is a fatal error
func IsFatal(err error) bool {
	_, isFatal := err.(*ConfigControllerFatalError)
	return isFatal
}

type BuildConfigController struct {
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	BuildConfigGetter       buildclient.BuildConfigGetter
	BuildLister             buildclient.BuildLister
	BuildDeleter            buildclient.BuildDeleter
	// recorder is used to record events.
	Recorder record.EventRecorder
}

func (c *BuildConfigController) HandleBuildConfig(bc *buildapi.BuildConfig) error {
	glog.V(4).Infof("Handling BuildConfig %s/%s", bc.Namespace, bc.Name)

	if err := buildutil.HandleBuildPruning(bc.Name, bc.Namespace, c.BuildLister, c.BuildConfigGetter, c.BuildDeleter); err != nil {
		utilruntime.HandleError(err)
	}

	hasChangeTrigger := buildapi.HasTriggerType(buildapi.ConfigChangeBuildTriggerType, bc)

	if !hasChangeTrigger {
		return nil
	}

	if bc.Status.LastVersion > 0 {
		return nil
	}

	glog.V(4).Infof("Running build for BuildConfig %s/%s", bc.Namespace, bc.Name)

	buildTriggerCauses := []buildapi.BuildTriggerCause{}
	// instantiate new build
	lastVersion := int64(0)
	request := &buildapi.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: buildapi.BuildTriggerCauseConfigMsg,
			}),
		ObjectMeta: metav1.ObjectMeta{
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
		} else if buildgenerator.IsFatal(err) || kerrors.IsNotFound(err) || kerrors.IsBadRequest(err) || kerrors.IsForbidden(err) {
			instantiateErr = fmt.Errorf("gave up on Build for BuildConfig %s/%s due to fatal error: %v", bc.Namespace, bc.Name, err)
			utilruntime.HandleError(instantiateErr)
			c.Recorder.Event(bc, kapi.EventTypeWarning, "BuildConfigInstantiateFailed", instantiateErr.Error())
			return &ConfigControllerFatalError{err.Error()}
		} else {
			instantiateErr = fmt.Errorf("error instantiating Build from BuildConfig %s/%s: %v", bc.Namespace, bc.Name, err)
			c.Recorder.Event(bc, kapi.EventTypeWarning, "BuildConfigInstantiateFailed", instantiateErr.Error())
			utilruntime.HandleError(instantiateErr)
		}
		return instantiateErr
	}
	return nil
}
