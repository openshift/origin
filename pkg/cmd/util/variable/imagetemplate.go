package variable

import (
	"fmt"
	"os"
	"strings"

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

const defaultImageFormat = "openshift3/ose-${component}:${version}"
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
