package controller

import (
	"fmt"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
)

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
			util.HandleError(instantiateErr)
		} else {
			instantiateErr = fmt.Errorf("error instantiating Build from BuildConfig %s/%s: %v", bc.Namespace, bc.Name, err)
			util.HandleError(instantiateErr)
		}
		return instantiateErr
	}
	return nil
}
