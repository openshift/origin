package imagesetup

import (
	"fmt"
	"os"

	"github.com/openshift/origin/test/extended/util/image"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

// OpenShift tests consume images from the cluster, from a number of vetted community locations,
// and from the upstream Kubernetes test suite which may reference images produced by a variety
// of build systems. To better organize and consolidate these images, the utility code within
// Kubernetes and OpenShift that is consulted to find image streams is made remappable - so that
// all images used by the test code can be located in one place. During normal operation the test
// images are read from the mirror, and administrators can choose to copy those images into
// restricted environment image registries and then run the tests against that subset. This also
// allows us to control the process whereby new images are introduced and review those in one spot.
//
// Test code utilizes helpers to get image strings throughout the code base, or is expected to use
// one or more of the images every OpenShift distribution includes in the 'openshift' namespace.
//
// See test/extended/util/image/README.md for a description of the process of adding a new image.

// DefaultTestImageMirrorLocation is where all Kube test inputs are sourced.
const DefaultTestImageMirrorLocation = "quay.io/openshift/community-e2e-images"

func VerifyTestImageRepoEnvVarUnset() error {
	if len(os.Getenv("KUBE_TEST_REPO")) > 0 {
		return fmt.Errorf("KUBE_TEST_REPO may not be specified when this command is run")
	}
	return nil
}

func VerifyImages() error {
	defaults := k8simage.GetOriginalImageConfigs()

	for originalPullSpec, index := range image.OriginalImages() {
		if index == -1 {
			continue
		}
		existing, ok := defaults[index]
		if !ok {
			return fmt.Errorf("image %q not found in upstream images, must be moved to test/extended/util/image.  Upstream mappings are:\n%v", originalPullSpec, defaults)
		}
		if existing.GetE2EImage() != originalPullSpec {
			return fmt.Errorf("image %q defines index %d but is defined upstream as %q, must be fixed in test/extended/util/image.  Upstream mappings are:\n%v", originalPullSpec, index, existing.GetE2EImage(), defaults)
		}
		// TODO remove after 2025 if still commented
		// We don't know what this code is for, but in case we discover that reason before 2025, I'm leaving this here
		//mirror := image.LocationFor(originalPullSpec)
		//upstreamMirror := k8simage.GetE2EImage(index)
		//if mirror != upstreamMirror {
		//	return fmt.Errorf("image %q defines index %d and mirror %q but is mirrored upstream as %q, must be fixed in test/extended/util/image.  Upstream mappings are:\n%v", originalPullSpec, index, mirror, upstreamMirror, defaults)
		//}
	}

	return nil
}
