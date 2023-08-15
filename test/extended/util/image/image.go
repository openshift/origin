package image

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

var (
	initializationLock sync.RWMutex
	initialized        bool
	fromRepository     string
	images             map[string]string

	allowedImages = map[string]k8simage.ImageID{
		// used by jenkins tests
		"quay.io/redhat-developer/nfs-server:1.1": -1,

		// used by open ldap tests
		"quay.io/openshifttest/ldap:1.2": -1,

		// used by multicast test
		"quay.io/openshifttest/multicast:1.1": -1,

		// used by oc mirror test, should be moved to publish to quay
		"docker.io/library/registry:2.8.0-beta.1": -1,

		// used by build s2i e2e's to verify that builder with USER root are not allowed
		// the github.com/openshift/build-test-images repo is built out of github.com/openshift/release
		"quay.io/redhat-developer/test-build-roots2i:1.2": -1,

		// used by all the rest build s2s e2e tests
		"quay.io/redhat-developer/test-build-simples2i:1.2": -1,

		// allowed upstream kube images - index and value must match upstream or
		// tests will fail (vendor/k8s.io/kubernetes/test/utils/image/manifest.go)
		"registry.k8s.io/e2e-test-images/agnhost:2.43": 1,
		"registry.k8s.io/e2e-test-images/nginx:1.15-4": 22,
	}
)

func getImages() map[string]string {
	initializationLock.RLock()
	if !initialized {
		fmt.Printf("Called getImages before initialization, starting wait.\n")
		initializationLock.RUnlock()

		for {
			time.Sleep(5 * time.Second)

			done := func() bool {
				initializationLock.RLock()
				defer initializationLock.RUnlock()

				if initialized {
					return true
				}
				return false
			}()
			if done {
				break
			}

			fmt.Printf("getImages still not initialized, waiting more.")
		}
	}
	return images
}

func InitializeImages(repo string) {
	initializationLock.Lock()
	defer initializationLock.Unlock()

	if initialized {
		panic(fmt.Sprintf("attempt to double initialize from %q to %q", fromRepository, repo))
	}
	initialized = true
	fromRepository = repo
	images = GetMappedImages(allowedImages, repo)
}

func GetGlobalFromRepository() string {
	return fromRepository
}

// ReplaceContents ensures that the provided yaml or json has the
// correct embedded image content.
func ReplaceContents(data []byte) ([]byte, error) {
	// exactImageFormat attempts to match a string on word boundaries
	const exactImageFormat = `\b%s\b`

	patterns := make(map[string]*regexp.Regexp)
	for from, to := range getImages() {
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
	pull, ok := getImages()[image]
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
	return LocationFor("quay.io/openshifttest/ldap:1.2")
}

// OriginalImages returns a map of the original image names.
func OriginalImages() map[string]k8simage.ImageID {
	images := make(map[string]k8simage.ImageID)
	for k, v := range allowedImages {
		images[k] = v
	}
	return images
}

// Exceptions is a list of images we don't mirror temporarily due to various
// problems. This list should ideally be empty.
var Exceptions = sets.NewString(
	"mcr.microsoft.com/windows:1809", // https://issues.redhat.com/browse/PROJQUAY-1874
	// this image has 3 windows/amd64 manifests, where layers are not compressed,
	// ie. application/vnd.docker.image.rootfs.diff.tar which are not accepted
	// by quay.io, this has to be manually mirrored with --filter-by-os=linux.*
	"registry.k8s.io/pause:3.8",
)

// GetMappedImages returns the images if they were mapped to the provided
// image repository. The keys of the returned map are the same as the keys
// in originalImages and the values are the equivalent name in the target
// repo.
func GetMappedImages(originalImages map[string]k8simage.ImageID, repo string) map[string]string {
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
