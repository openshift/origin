package validation

import (
	"strings"

	"k8s.io/kubernetes/pkg/util/parsers"
)

// ParseDomainName parses a docker image reference into its domain
// component, if any, and everything after the domain. An empty string
// is returned if there is no domain component. This function will
// first validate that image is a valid reference, returning an error
// if it is not.
//
// Examples inputs and results for the domain component:
//
//   "busybox"                      -> domain is ""
//   "foo/busybox"                  -> domain is ""
//   "localhost/foo/busybox"        -> domain is "localhost"
//   "localhost:5000/foo/busybox"   -> domain is "localhost:5000"
//   "gcr.io/busybox"               -> domain is "gcr.io"
//   "gcr.io/foo/busybox"           -> domain is "gcr.io"
//   "docker.io/busybox"            -> domain is "docker.io"
//   "docker.io/library/busybox"    -> domain is "docker.io"
//   "library/busybox:v1"           -> domain is ""
func ParseDomainName(image string) (string, string, error) {
	// Note: when we call ParseImageName() this gets normalized to
	// potentially include "docker.io", and/or "library/" and or
	// "latest". We are only interested in discerning the domain
	// and the remainder based on the non-normalised reference. If
	// the image is valid we do our own parsing of the first
	// component (i.e., the repository) to see if it actually
	// reflects a domain name.
	if _, _, _, err := parsers.ParseImageName(image); err != nil {
		return "", "", err
	}
	i := strings.IndexRune(image, '/')
	if i == -1 || (!strings.ContainsAny(image[:i], ".:") && image[:i] != "localhost") {
		return "", image, nil
	}
	return image[:i], image[i+1:], nil
}
