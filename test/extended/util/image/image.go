package image

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func init() {
	allowedImages = map[string]int{
		// used by open ldap tests
		"docker.io/mrogers950/origin-openldap-test:fedora29": -1,

		// used by multicast test, should be moved to publish to quay
		"docker.io/openshift/test-multicast:latest": -1,

		// used by oc mirror test, should be moved to publish to quay
		"docker.io/library/registry:2.7.1": -1,

		// moved to GCR
		"k8s.gcr.io/sig-storage/csi-attacher:v2.2.0":              -1,
		"k8s.gcr.io/sig-storage/csi-attacher:v3.0.0":              -1,
		"k8s.gcr.io/sig-storage/csi-node-driver-registrar:v1.2.0": -1,
		"k8s.gcr.io/sig-storage/csi-node-driver-registrar:v1.3.0": -1,
		"k8s.gcr.io/sig-storage/csi-provisioner:v1.6.0":           -1,
		"k8s.gcr.io/sig-storage/csi-provisioner:v2.0.0":           -1,
		"k8s.gcr.io/sig-storage/csi-resizer:v0.4.0":               -1,
		"k8s.gcr.io/sig-storage/csi-resizer:v0.5.0":               -1,
		"k8s.gcr.io/sig-storage/csi-snapshotter:v2.0.1":           -1,
		"k8s.gcr.io/sig-storage/csi-snapshotter:v2.1.0":           -1,
		"k8s.gcr.io/sig-storage/hostpathplugin:v1.4.0":            -1,
		"k8s.gcr.io/sig-storage/livenessprobe:v1.1.0":             -1,
		"k8s.gcr.io/sig-storage/mock-driver:v4.0.2":               -1,
		"k8s.gcr.io/sig-storage/snapshot-controller:v2.1.1":       -1,

		// allowed upstream kube images - index and value must match upstream or
		// tests will fail
		"k8s.gcr.io/e2e-test-images/agnhost:2.21": 1,
		"docker.io/library/nginx:1.14-alpine":     23,
		"docker.io/library/nginx:1.15-alpine":     24,
		"docker.io/library/redis:5.0.5-alpine":    31,
	}

	images = GetMappedImages(allowedImages, os.Getenv("KUBE_TEST_REPO"))
}

var (
	images        map[string]string
	allowedImages map[string]int
)

// ReplaceContents ensures that the provided yaml or json has the
// correct embedded image content.
func ReplaceContents(data []byte) ([]byte, error) {
	// exactImageFormat attempts to match a string on word boundaries
	const exactImageFormat = `\b%s\b`

	patterns := make(map[string]*regexp.Regexp)
	for from, to := range images {
		pattern := fmt.Sprintf(exactImageFormat, regexp.QuoteMeta(from))
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		patterns[to] = re
	}

	for to, pattern := range patterns {
		data = pattern.ReplaceAll(data, []byte(to))
	}

	return data, nil
}

// MustReplaceContents invokes ReplaceContents and panics if any
// replacement error occurs.
func MustReplaceContents(data []byte) []byte {
	data, err := ReplaceContents(data)
	if err != nil {
		panic(err)
	}
	return data
}

// LocationFor returns the appropriate URL for the provided image.
func LocationFor(image string) string {
	pull, ok := images[image]
	if !ok {
		panic(fmt.Sprintf(`The image %q is not one of the pre-approved test images.

To add a new image to OpenShift tests you must follow the process described in
the test/extended/util/image/README.md file.`, image))
	}
	return pull
}

// ShellImage returns a docker pull spec that any pod on the cluster
// has access to that contains bash and standard commandline tools.
// This image should be used for all generic e2e shell scripts. This
// image has oc.
//
// If the script you are running does not have a specific tool available
// that is required, open an issue to github.com/openshift/images in the
// images/tools directory to discuss adding that package. In general, try
// to avoid the need to add packages by using simpler concepts or consider
// extending an existing image.
func ShellImage() string {
	return "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest"
}

// LimitedShellImage returns a docker pull spec that any pod on the cluster
// has access to that contains bash and standard commandline tools.
// This image should be used when you only need oc and can't use the shell image.
// This image has oc.
//
// TODO: this will be removed when https://bugzilla.redhat.com/show_bug.cgi?id=1843232
// is fixed
func LimitedShellImage() string {
	return "image-registry.openshift-image-registry.svc:5000/openshift/cli:latest"
}

// OpenLDAPTestImage returns the LDAP test image.
func OpenLDAPTestImage() string {
	return LocationFor("docker.io/mrogers950/origin-openldap-test:fedora29")
}

// OriginalImages returns a map of the original image names.
func OriginalImages() map[string]int {
	images := make(map[string]int)
	for k, v := range allowedImages {
		images[k] = v
	}
	return images
}

// Images returns a map of all images known to the test package.
func Images() map[string]struct{} {
	copied := make(map[string]struct{})
	for k := range images {
		copied[k] = struct{}{}
	}
	return copied
}

// GetMappedImages returns the images if they were mapped to the provided
// image repository. The keys of the returned map are the same as the keys
// in originalImages and the values are the equivalent name in the target
// repo.
func GetMappedImages(originalImages map[string]int, repo string) map[string]string {
	if len(repo) == 0 {
		images := make(map[string]string)
		for k := range originalImages {
			images[k] = k
		}
		return images
	}
	configs := make(map[string]string)
	reCharSafe := regexp.MustCompile(`[^\w]`)
	reDashes := regexp.MustCompile(`-+`)
	h := sha256.New()

	const (
		// length of hash in base64-url chosen to minimize possible collisions (64^16 possible)
		hashLength = 16
		// maximum length of a Docker spec image tag
		maxTagLength = 127
		// when building a tag, there are at most 6 characters in the format (e2e and 3 dashes),
		// and we should allow up to 10 digits for the index and additional qualifiers we may add
		// in the future
		tagFormatCharacters = 6 + 10
	)

	parts := strings.SplitN(repo, "/", 2)
	registry, destination := parts[0], parts[1]
	for pullSpec, index := range originalImages {
		// Build a new tag with a the index, a hash of the image spec (to be unique) and
		// shorten and make the pull spec "safe" so it will fit in the tag
		h.Reset()
		h.Write([]byte(pullSpec))
		hash := base64.RawURLEncoding.EncodeToString(h.Sum(nil))[:hashLength]
		shortName := reCharSafe.ReplaceAllLiteralString(pullSpec, "-")
		shortName = reDashes.ReplaceAllLiteralString(shortName, "-")
		maxLength := maxTagLength - hashLength - tagFormatCharacters
		if len(shortName) > maxLength {
			shortName = shortName[len(shortName)-maxLength:]
		}
		var newTag string
		if index == -1 {
			newTag = fmt.Sprintf("e2e-%s-%s", shortName, hash)
		} else {
			newTag = fmt.Sprintf("e2e-%d-%s-%s", index, shortName, hash)
		}

		configs[pullSpec] = fmt.Sprintf("%s/%s:%s", registry, destination, newTag)
	}
	return configs
}
