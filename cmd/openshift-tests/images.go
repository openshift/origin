package main

import (
	"fmt"
	"sort"

	"github.com/openshift/library-go/pkg/image/reference"

	k8simage "k8s.io/kubernetes/test/utils/image"
)

// defaultTestImageMirrorLocation is where all Kube test inputs are sourced.
const defaultTestImageMirrorLocation = "quay.io/openshift/community-e2e-images"

// createImageMirrorForInternalImages returns a list of 'oc image mirror' mappings from source to
// target or returns an error. If mirrored is true the images are assumed to be in the REPO:TAG
// format where TAG is a hash of the original internal name and the index of the image in the
// array. Otherwise the mirror target will have the expected hash.
func createImageMirrorForInternalImages(prefix string, ref reference.DockerImageReference, mirrored bool) ([]string, error) {
	defaults := k8simage.GetOriginalImageConfigs()
	updated := k8simage.GetMappedImageConfigs(defaults, ref.String())

	// if we've mirrored, then the source is going to be our repo, not upstream's
	if mirrored {
		baseRef, err := reference.Parse(defaultTestImageMirrorLocation)
		if err != nil {
			return nil, fmt.Errorf("invalid default mirror location: %v", err)
		}
		for i, config := range updated {
			defaultConfig := defaults[i]
			pullSpec := config.GetE2EImage()
			if pullSpec == defaultConfig.GetE2EImage() {
				continue
			}
			e2eRef, err := reference.Parse(pullSpec)
			if err != nil {
				return nil, fmt.Errorf("invalid test image: %s: %v", pullSpec, err)
			}
			if len(e2eRef.Tag) == 0 {
				return nil, fmt.Errorf("invalid test image: %s: no tag", pullSpec)
			}
			config.SetRegistry(baseRef.Registry)
			config.SetName(baseRef.RepositoryName())
			config.SetVersion(e2eRef.Tag)
			defaults[i] = config
		}
	}

	var lines []string
	for i := range updated {
		if i == k8simage.StartupScript {
			continue
		}
		a, b := defaults[i], updated[i]
		from, to := a.GetE2EImage(), b.GetE2EImage()
		if from == to {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s%s", from, prefix, to))
	}
	sort.Strings(lines)
	return lines, nil
}
