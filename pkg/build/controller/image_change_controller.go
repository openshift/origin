package controller

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kerrors "k8s.io/kubernetes/pkg/api/errors"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/util"

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

	// Loop through all build configurations and record if there was an error
	// instead of breaking the loop. The error will be returned in the end, so the
	// retry controller can retry. Any BuildConfigs that were processed successfully
	// should have had their LastTriggeredImageID updated, so the retry should result
	// in a no-op for them.
	hasError := false

	// TODO: this is inefficient
	for _, bc := range c.BuildConfigStore.List() {
		config := bc.(*buildapi.BuildConfig)

		var (
			from           *kapi.ObjectReference
			shouldBuild    = false
			triggeredImage = ""
		)
		// For every ImageChange trigger find the latest tagged image from the image repository and
		// invoke a build using that image id. A new build is triggered only if the latest tagged image id or pull spec
		// differs from the last triggered build recorded on the build config for that trigger
		for _, trigger := range config.Spec.Triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				continue
			}
			if trigger.ImageChange.From != nil {
				from = trigger.ImageChange.From
			} else {
				from = buildutil.GetImageStreamForStrategy(config.Spec.Strategy)
			}

			if from == nil || from.Kind != "ImageStreamTag" {
				continue
			}
			fromStreamName, tag, ok := imageapi.SplitImageStreamTag(from.Name)
			if !ok {
				glog.Errorf("Invalid image stream tag: %s in build config %s/%s", from.Name, config.Name, config.Namespace)
				continue
			}

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
			latest := imageapi.LatestTaggedImage(repo, tag)
			if latest == nil {
				glog.V(4).Infof("unable to find tagged image: no image recorded for %s/%s:%s", repo.Namespace, repo.Name, tag)
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
					Name:      config.Name,
					Namespace: config.Namespace,
				},
				TriggeredByImage: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: triggeredImage,
				},
				From: from,
			}
			if _, err := c.BuildConfigInstantiator.Instantiate(config.Namespace, request); err != nil {
				if kerrors.IsConflict(err) {
					util.HandleError(fmt.Errorf("unable to instantiate Build for BuildConfig %s/%s due to a conflicting update: %v", config.Namespace, config.Name, err))
				} else {
					util.HandleError(fmt.Errorf("error instantiating Build from BuildConfig %s/%s: %v", config.Namespace, config.Name, err))
				}
				hasError = true
				continue
			}
		}
	}
	if hasError {
		return fmt.Errorf("an error occurred processing 1 or more build configurations; the image change trigger for image stream %s will be retried", repo.Status.DockerImageRepository)
	}
	return nil
}
