package controller

import (
	"fmt"
	"strings"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageChangeControllerFatalError represents a fatal error while handling an image change
type ImageChangeControllerFatalError struct {
	Reason string
	Err    error
}

func (e ImageChangeControllerFatalError) Error() string {
	return fmt.Sprintf("fatal error handling ImageStream change: %s, root error was: %s", e.Reason, e.Err.Error())
}

// ImageChangeController watches for changes to ImageRepositories and triggers
// builds when a new version of a tag referenced by a BuildConfig
// is available.
type ImageChangeController struct {
	BuildConfigStore        cache.Store
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	// Stop is an optional channel that controls when the controller exits
	Stop <-chan struct{}
}

// getImageStreamNameFromReference strips off the :tag or @id suffix
// from an ImageStream[Tag,Image,''].Name
func getImageStreamNameFromReference(ref *kapi.ObjectReference) string {
	name := strings.Split(ref.Name, ":")[0]
	return strings.Split(name, "@")[0]
}

// HandleImageRepo processes the next ImageStream event.
func (c *ImageChangeController) HandleImageRepo(repo *imageapi.ImageStream) error {
	glog.V(4).Infof("Build image change controller detected ImageStream change %s", repo.Status.DockerImageRepository)

	// TODO: this is inefficient
	for _, bc := range c.BuildConfigStore.List() {
		config := bc.(*buildapi.BuildConfig)

		from := buildutil.GetImageStreamForStrategy(config.Parameters.Strategy)
		if from == nil || from.Kind != "ImageStreamTag" {
			continue
		}

		shouldBuild := false
		triggeredImage := ""
		// For every ImageChange trigger find the latest tagged image from the image repository and replace that value
		// throughout the build strategies. A new build is triggered only if the latest tagged image id or pull spec
		// differs from the last triggered build recorded on the build config.
		for _, trigger := range config.Triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				continue
			}
			fromStreamName := getImageStreamNameFromReference(from)

			fromNamespace := from.Namespace
			if len(fromNamespace) == 0 {
				fromNamespace = config.Namespace
			}

			// only trigger a build if this image repo matches the name and namespace of the ref in the build trigger
			// also do not trigger if the imagerepo does not have a valid DockerImageRepository value for us to pull
			// the image from
			if len(repo.Status.DockerImageRepository) == 0 || fromStreamName != repo.Name || fromNamespace != repo.Namespace {
				continue
			}

			// This split is safe because ImageStreamTag names always have the form
			// name:tag.
			tag := strings.Split(from.Name, ":")[1]
			latest := imageapi.LatestTaggedImage(repo, tag)
			if latest == nil {
				util.HandleError(fmt.Errorf("unable to find tagged image: no image recorded for %s/%s:%s", repo.Namespace, repo.Name, tag))
				continue
			}
			glog.V(4).Infof("Found ImageStream %s/%s with tag %s", repo.Namespace, repo.Name, tag)

			// (must be different) to trigger a build
			last := trigger.ImageChange.LastTriggeredImageID
			next := latest.DockerImageReference

			if len(last) == 0 || (len(next) > 0 && next != last) {
				triggeredImage = next
				shouldBuild = true
				// it doesn't really make sense to have multiple image change triggers any more,
				// so just exit the loop now
				break
			}
		}

		if shouldBuild {
			glog.V(4).Infof("Running build for BuildConfig %s/%s", config.Namespace, config.Name)
			// instantiate new build
			request := &buildapi.BuildRequest{
				ObjectMeta: kapi.ObjectMeta{
					Name: config.Name,
				},
				TriggeredByImage: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: triggeredImage,
				},
			}
			if _, err := c.BuildConfigInstantiator.Instantiate(config.Namespace, request); err != nil {
				if kerrors.IsConflict(err) {
					// This is not a retryable error. The worst case outcome of not updating the buildconfig
					// is that we might rerun a build for the same "new" imageid change in the future,
					// which is better than guaranteeing we run the build 2+ times by retrying it here.
					return ImageChangeControllerFatalError{
						Reason: fmt.Sprintf("unable to instantiate Build for BuildConfig %s/%s due to a conflicting update: %v", config.Namespace, config.Name, err),
						Err:    err,
					}
				}

				return fmt.Errorf("error instantiating Build from BuildConfig %s/%s: %v", config.Namespace, config.Name, err)
			}
		}
	}
	return nil
}
