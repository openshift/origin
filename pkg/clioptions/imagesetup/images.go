package imagesetup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/test/extended/util/image"
	"k8s.io/kube-openapi/pkg/util/sets"
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
	}

	return nil
}

// VerifyManifestLists verifies that all source images have manifest lists
// containing all required architectures (amd64, arm64, ppc64le, s390x).
// Images in the allowedExceptions list are skipped from verification.
func VerifyManifestLists(sourceImages []string, allowedExceptions []string) error {
	requiredArchs := []string{"amd64", "arm64", "ppc64le", "s390x"}

	logrus.Infof("Verifying manifest lists for %d unique images...", len(sourceImages))

	var problematicImages []string
	var skippedImages []string
	for i, img := range sourceImages {
		logrus.Debugf("[%d/%d] Checking image: %s", i+1, len(sourceImages), img)

		if isImageExcepted(img, allowedExceptions) {
			logrus.Debugf("  Image is in exception list, skipping")
			skippedImages = append(skippedImages, img)
			continue
		}

		availableArchs, err := getArchitectures(img, requiredArchs)
		if err != nil {
			return fmt.Errorf("failed to get image info for %s: %w", img, err)
		}

		logrus.Debugf("  Found %d architecture(s)", len(availableArchs))

		var missingArchs []string
		availableSet := sets.NewString(availableArchs...)
		for _, required := range requiredArchs {
			if !availableSet.Has(required) {
				missingArchs = append(missingArchs, required)
			}
		}

		if len(missingArchs) > 0 {
			problematicImages = append(problematicImages,
				fmt.Sprintf("%s: missing architectures: %s (has: %s)",
					img, strings.Join(missingArchs, ", "), strings.Join(availableArchs, ", ")))
		}
	}

	if len(skippedImages) > 0 {
		logrus.Infof("Skipped %d image(s) from verification (allowed exceptions)", len(skippedImages))
	}

	if len(problematicImages) > 0 {
		return fmt.Errorf("the following images do not have manifest lists with all required architectures (amd64, arm64, ppc64le, s390x):\n  %s",
			strings.Join(problematicImages, "\n  "))
	}

	logrus.Infof("All images have manifest lists with required architectures.")
	return nil
}

// isImageExcepted checks if an image matches any of the allowed exceptions using substring matching.
func isImageExcepted(image string, exceptions []string) bool {
	for _, exception := range exceptions {
		if strings.Contains(image, exception) {
			return true
		}
	}
	return false
}

// getArchitectures retrieves image information using 'oc image info' with --filter-by-os
// to check which architectures are available for the image. Returns a list of available architectures.
// Retries up to 3 times per architecture to handle transient failures.
func getArchitectures(image string, requiredArchs []string) ([]string, error) {
	var availableArchs []string
	const maxRetries = 3

	for _, arch := range requiredArchs {
		logrus.Debugf("    Checking architecture: %s", arch)

		found := false
		for attempt := 1; attempt <= maxRetries; attempt++ {
			cmd := exec.Command("oc", "image", "info", image,
				fmt.Sprintf("--filter-by-os=linux/%s", arch), "--output=json")
			err := cmd.Run()

			if err == nil {
				logrus.Debugf("      ✓ %s available", arch)
				availableArchs = append(availableArchs, arch)
				found = true
				break
			}

			if attempt < maxRetries {
				logrus.Debugf("      ✗ %s not found (attempt %d/%d), retrying...", arch, attempt, maxRetries)
				time.Sleep(time.Second)
			}
		}

		if !found {
			logrus.Debugf("      ✗ %s not available after %d attempts", arch, maxRetries)
		}
	}

	if len(availableArchs) == 0 {
		return nil, fmt.Errorf("no supported architectures found for image %s", image)
	}

	return availableArchs, nil
}
