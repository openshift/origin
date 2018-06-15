package app

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/kubernetes/pkg/version"

	"github.com/golang/glog"
)

// ImageTemplate is a class to assist in expanding parameterized Docker image references
// from configuration or a file
type ImageTemplate struct {
	// Format is required, set to the image template to pull
	Format string
	Latest bool
	// EnvFormat is optional, if set will substitute the value of ${component} with any env
	// var that matches this format. Is a printf format string accepting a single
	// string parameter.
	EnvFormat string
}

var (
	// defaultImagePrefix is the default prefix for any container image names.
	// This value should be set duing build via -ldflags.
	DefaultImagePrefix string

	// defaultImageFormat is the default format for container image names used
	// to run containerized components of the platform
	defaultImageFormat = DefaultImagePrefix + "-${component}:${version}"
)

const defaultImageEnvFormat = "OPENSHIFT_%s_IMAGE"

// NewDefaultImageTemplate returns the default image template
func NewDefaultImageTemplate() ImageTemplate {
	return ImageTemplate{
		Format:    defaultImageFormat,
		Latest:    false,
		EnvFormat: defaultImageEnvFormat,
	}
}

// ExpandOrDie will either expand a string or exit in case of failure
func (t *ImageTemplate) ExpandOrDie(component string) string {
	value, err := t.Expand(component)
	if err != nil {
		glog.Fatalf("Unable to find an image for %q due to an error processing the format: %v", component, err)
	}
	return value
}

// Expand expands a string using a series of common format functions
func (t *ImageTemplate) Expand(component string) (string, error) {
	template := t.Format
	if len(t.EnvFormat) > 0 {
		if s, ok := t.imageComponentEnvExpander(component); ok {
			template = s
		}
	}
	value, err := ExpandStrict(template, func(key string) (string, bool) {
		switch key {
		case "component":
			return component, true
		case "version":
			if t.Latest {
				return "latest", true
			}
		}
		return "", false
	}, Versions)
	return value, err
}

func (t *ImageTemplate) imageComponentEnvExpander(key string) (string, bool) {
	s := strings.Replace(strings.ToUpper(key), "-", "_", -1)
	val := os.Getenv(fmt.Sprintf(t.EnvFormat, s))
	if len(val) == 0 {
		return "", false
	}
	return val, true
}

// ExpandStrict expands a string using a series of common format functions
func ExpandStrict(s string, fns ...KeyFunc) (string, error) {
	unmatched := []string{}
	result := os.Expand(s, func(key string) string {
		for _, fn := range fns {
			val, ok := fn(key)
			if !ok {
				continue
			}
			return val
		}
		unmatched = append(unmatched, key)
		return ""
	})

	switch len(unmatched) {
	case 0:
		return result, nil
	case 1:
		return "", fmt.Errorf("the key %q in %q is not recognized", unmatched[0], s)
	default:
		return "", fmt.Errorf("multiple keys in %q were not recognized: %s", s, strings.Join(unmatched, ", "))
	}
}

// KeyFunc returns the value associated with the provided key or false if no
// such key exists.
type KeyFunc func(key string) (string, bool)

// Versions is a KeyFunc for retrieving information about the current version.
func Versions(key string) (string, bool) {
	switch key {
	case "shortcommit":
		s := OverrideVersion.GitCommit
		if len(s) > 7 {
			s = s[:7]
		}
		return s, true
	case "version":
		s := lastSemanticVersion(OverrideVersion.GitVersion)
		return s, true
	default:
		return "", false
	}
}

// OverrideVersion is the latest version, exposed for testing.
var OverrideVersion = version.Get()

// lastSemanticVersion attempts to return a semantic version from the GitVersion - which
// is either <semver>+<commit> or <semver> on release boundaries.
func lastSemanticVersion(gitVersion string) string {
	parts := strings.Split(gitVersion, "+")
	return parts[0]
}
