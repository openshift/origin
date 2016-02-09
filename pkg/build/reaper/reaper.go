package reaper

import (
	"strings"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/util"
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
func (reaper *BuildConfigReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *kapi.DeleteOptions) error {
	noBcFound := false
	noBuildFound := true

	// Add deletion pending annotation to the build config
	err := unversioned.RetryOnConflict(unversioned.DefaultRetry, func() error {
		bc, err := reaper.oc.BuildConfigs(namespace).Get(name)
		if kerrors.IsNotFound(err) {
			noBcFound = true
			return nil
		}
		if err != nil {
			return err
		}

		// Ignore if the annotation already exists
		if strings.ToLower(bc.Annotations[buildapi.BuildConfigPausedAnnotation]) == "true" {
			return nil
		}

		// Set the annotation and update
		if err := util.AddObjectAnnotations(bc, map[string]string{buildapi.BuildConfigPausedAnnotation: "true"}); err != nil {
			return err
		}
		_, err = reaper.oc.BuildConfigs(namespace).Update(bc)
		return err
	})
	if err != nil {
		return err
	}

	// Warn the user if the BuildConfig won't get deleted after this point.
	bcDeleted := false
	defer func() {
		if !bcDeleted {
			glog.Warningf("BuildConfig %s/%s will not be deleted because not all associated builds could be deleted. You can try re-running the command or removing them manually", namespace, name)
		}
	}()

	// Collect builds related to the config.
	builds, err := reaper.oc.Builds(namespace).List(kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelector(name)})
	if err != nil {
		return err
	}
	errList := []error{}
	for _, build := range builds.Items {
		noBuildFound = false
		if err := reaper.oc.Builds(namespace).Delete(build.Name); err != nil {
			glog.Warningf("Cannot delete Build %s/%s: %v", build.Namespace, build.Name, err)
			if !kerrors.IsNotFound(err) {
				errList = append(errList, err)
			}
		}
	}

	// Collect deprecated builds related to the config.
	// TODO: Delete this block after BuildConfigLabelDeprecated is removed.
	builds, err = reaper.oc.Builds(namespace).List(kapi.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(name)})
	if err != nil {
		return err
	}
	for _, build := range builds.Items {
		noBuildFound = false
		if err := reaper.oc.Builds(namespace).Delete(build.Name); err != nil {
			glog.Warningf("Cannot delete Build %s/%s: %v", build.Namespace, build.Name, err)
			if !kerrors.IsNotFound(err) {
				errList = append(errList, err)
			}
		}
	}

	// Aggregate all errors
	if len(errList) > 0 {
		return kutilerrors.NewAggregate(errList)
	}

	// Finally we can delete the BuildConfig
	if !noBcFound {
		if err := reaper.oc.BuildConfigs(namespace).Delete(name); err != nil {
			return err
		}
	}
	bcDeleted = true

	if noBcFound && noBuildFound {
		return kerrors.NewNotFound(buildapi.Resource("buildconfig"), name)
	}

	return nil
}
