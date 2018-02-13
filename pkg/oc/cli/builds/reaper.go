package builds

import (
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/kubectl"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/util"
)

// NewBuildConfigReaper returns a new reaper for buildConfigs
func NewBuildConfigReaper(buildClient buildclient.Interface) kubectl.Reaper {
	return &BuildConfigReaper{buildClient: buildClient, pollInterval: kubectl.Interval, timeout: kubectl.Timeout}
}

// BuildConfigReaper implements the Reaper interface for buildConfigs
type BuildConfigReaper struct {
	buildClient           buildclient.Interface
	pollInterval, timeout time.Duration
}

// Stop deletes the build configuration and all of the associated builds.
func (reaper *BuildConfigReaper) Stop(namespace, name string, timeout time.Duration, gracePeriod *metav1.DeleteOptions) error {
	_, err := reaper.buildClient.Build().BuildConfigs(namespace).Get(name, metav1.GetOptions{})

	if err != nil {
		return err
	}

	var bcPotentialBuilds []buildapi.Build

	// Collect builds related to the config.
	builds, err := reaper.buildClient.Build().Builds(namespace).List(metav1.ListOptions{LabelSelector: buildutil.BuildConfigSelector(name).String()})
	if err != nil {
		return err
	}

	bcPotentialBuilds = append(bcPotentialBuilds, builds.Items...)

	// Collect deprecated builds related to the config.
	// TODO: Delete this block after BuildConfigLabelDeprecated is removed.
	builds, err = reaper.buildClient.Build().Builds(namespace).List(metav1.ListOptions{LabelSelector: buildutil.BuildConfigSelectorDeprecated(name).String()})
	if err != nil {
		return err
	}

	bcPotentialBuilds = append(bcPotentialBuilds, builds.Items...)

	// A map of builds associated with this build configuration
	bcBuilds := make(map[ktypes.UID]buildapi.Build)

	// Because of name length limits in the BuildConfigSelector, annotations are used to ensure
	// reliable selection of associated builds.
	for _, build := range bcPotentialBuilds {
		if build.Annotations != nil {
			if bcName, ok := build.Annotations[buildapi.BuildConfigAnnotation]; ok {
				// The annotation, if present, has the full build config name.
				if bcName != name {
					// If the name does not match exactly, the build is not truly associated with the build configuration
					continue
				}
			}
		}
		// Note that if there is no annotation, this is a deprecated build spec
		// and we choose to include it in the deletion having matched only the BuildConfigSelectorDeprecated

		// Use a map to union the lists returned by the contemporary & deprecated build queries
		// (there will be overlap between the lists, and we only want to try to delete each build once)
		bcBuilds[build.UID] = build
	}

	// If there are builds associated with this build configuration, pause it before attempting the deletion
	if len(bcBuilds) > 0 {

		// Add paused annotation to the build config pending the deletion
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {

			bc, err := reaper.buildClient.Build().BuildConfigs(namespace).Get(name, metav1.GetOptions{})
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
			_, err = reaper.buildClient.Build().BuildConfigs(namespace).Update(bc)
			return err
		})

		if err != nil {
			return err
		}

	}

	// Warn the user if the BuildConfig won't get deleted after this point.
	bcDeleted := false
	defer func() {
		if !bcDeleted {
			glog.Warningf("BuildConfig %s/%s will not be deleted because not all associated builds could be deleted. You can try re-running the command or removing them manually", namespace, name)
		}
	}()

	// For the benefit of test cases, sort the UIDs so that the deletion order is deterministic
	buildUIDs := make([]string, 0, len(bcBuilds))
	for buildUID := range bcBuilds {
		buildUIDs = append(buildUIDs, string(buildUID))
	}
	sort.Strings(buildUIDs)

	errList := []error{}
	for _, buildUID := range buildUIDs {
		build := bcBuilds[ktypes.UID(buildUID)]
		if err := reaper.buildClient.Build().Builds(namespace).Delete(build.Name, &metav1.DeleteOptions{}); err != nil {
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

	if err := reaper.buildClient.Build().BuildConfigs(namespace).Delete(name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	bcDeleted = true
	return nil
}
