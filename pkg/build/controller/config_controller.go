package controller

import (
	"fmt"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	kutil "k8s.io/kubernetes/pkg/util"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	"github.com/openshift/origin/pkg/build/util"
)

type BuildConfigController struct {
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	BuildConfigUpdater      buildclient.BuildConfigUpdater
	BuildConfigDeleter      buildclient.BuildConfigDeleter
	BuildDeleter            buildclient.BuildDeleter
	BuildLister             buildclient.BuildLister
}

func (c *BuildConfigController) HandleBuildConfig(bc *buildapi.BuildConfig) error {
	glog.V(4).Infof("Handling BuildConfig %s/%s", bc.Namespace, bc.Name)

	if !bc.DeletionTimestamp.IsZero() {
		if bc.Status.CanDelete {
			return nil
		}

		builds, err := c.BuildLister.List(bc.Namespace, util.ConfigSelector(bc.Name), nil)
		if err != nil {
			return err
		}

		buildsWithDeprecatedLabel, err := c.BuildLister.List(bc.Namespace, util.ConfigSelectorDeprecated(bc.Name), nil)
		if err != nil {
			return err
		}

		var errlist []error
		buildList := append(builds.Items, buildsWithDeprecatedLabel.Items...)
		for _, build := range buildList {
			if err = c.BuildDeleter.Delete(build.Namespace, build.Name); err != nil {
				glog.Errorf("Cannot delete Build %s/%s: %v", build.Namespace, build.Name, err)
				errlist = append(errlist, err)
			}
		}
		err = kerrors.NewAggregate(errlist)
		if err != nil {
			return kapierrors.NewInternalError(err)
		}

		bc.Status.CanDelete = true
		err = c.BuildConfigUpdater.Update(bc)
		if err != nil {
			if kapierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return c.BuildConfigDeleter.Delete(bc.Namespace, bc.Name)
	}

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
		if kapierrors.IsConflict(err) {
			instantiateErr = fmt.Errorf("unable to instantiate Build for BuildConfig %s/%s due to a conflicting update: %v", bc.Namespace, bc.Name, err)
			kutil.HandleError(instantiateErr)
		} else {
			instantiateErr = fmt.Errorf("error instantiating Build from BuildConfig %s/%s: %v", bc.Namespace, bc.Name, err)
			kutil.HandleError(instantiateErr)
		}
		return instantiateErr
	}
	return nil
}
