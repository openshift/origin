package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"

	"github.com/openshift/library-go/pkg/image/reference"

	k8simage "k8s.io/kubernetes/test/utils/image"
)

// defaultTestImageMirrorLocation is where all Kube test inputs are sourced.
const defaultTestImageMirrorLocation = "quay.io/openshift/community-e2e-images"

var updateImagesOnce bool

// updateInternalImages sets all of the e2e test image locations as if they had been
// mirrored to repository. This command can only be run once per process.
func updateInternalImages(repository string) error {
	if len(repository) == 0 {
		return nil
	}
	if updateImagesOnce {
		return fmt.Errorf("updateInternalImages may be called only once per process")
	}
	updateImagesOnce = true

	h := sha256.New()
	m := k8simage.GetImageConfigs()
	ref, err := reference.Parse(repository)
	if err != nil {
		return fmt.Errorf("the test image repository name must be valid: %v", err)
	}
	for i, image := range m {
		h.Reset()
		h.Write([]byte(image.GetE2EImage()))
		tag := fmt.Sprintf("e2e_%s_%d", base64.RawURLEncoding.EncodeToString(h.Sum(nil))[:10], i)
		ref.Tag = tag
		image.SetRegistry(ref.Registry)
		image.SetName(ref.RepositoryName())
		image.SetVersion(ref.Tag)
		m[i] = image
	}
	return nil
}

// createImageMirrorForInternalImages returns a list of 'oc image mirror' mappings from source to
// target or returns an error. If mirrored is true the images are assumed to be in the REPO:TAG
// format where TAG is a hash of the original internal name and the index of the image in the
// array. Otherwise the mirror target will have the expected hash.
func createImageMirrorForInternalImages(ref reference.DockerImageReference, mirrored bool) ([]string, error) {
	var lines []string
	for i, image := range k8simage.GetImageConfigs() {
		// these images have special case behavior
		switch i {
		case k8simage.InvalidRegistryImage, k8simage.AuthenticatedAlpine,
			k8simage.AuthenticatedWindowsNanoServer, k8simage.AgnhostPrivate:
			// these images cannot be mirrored (either they don't exist, or depend on specific permissions and are test only)
			continue
		case k8simage.StartupScript:
			// this image was replaced with busybox in kubernetes/kubernetes#89556
			image = k8simage.GetConfig(k8simage.BusyBox)
		default:
		}

		// determine the target location
		h := sha256.New()
		pullSpec := image.GetE2EImage()
		if mirrored {
			e2eRef, err := reference.Parse(pullSpec)
			if err != nil {
				return nil, fmt.Errorf("invalid test image: %s: %v", pullSpec, err)
			}
			if len(e2eRef.Tag) == 0 {
				return nil, fmt.Errorf("invalid test image: %s: no tag", pullSpec)
			}
			ref.Tag = e2eRef.Tag
		} else {
			h.Reset()
			h.Write([]byte(pullSpec))
			tag := fmt.Sprintf("e2e_%s_%d", base64.RawURLEncoding.EncodeToString(h.Sum(nil))[:10], i)
			ref.Tag = tag
		}
		lines = append(lines, fmt.Sprintf("%s %s", pullSpec, ref.String()))
	}

	sort.Strings(lines)
	return lines, nil
}
