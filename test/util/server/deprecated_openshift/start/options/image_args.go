package options

import (
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

// ImageFormatArgs is a struct that the command stores flag values into.
type ImageFormatArgs struct {
	// ImageTemplate is used in expanding parameterized Docker image references
	// from configuration or a file
	ImageTemplate variable.ImageTemplate
}

// NewDefaultImageFormatArgs returns the default image template
func NewDefaultImageFormatArgs() *ImageFormatArgs {
	config := &ImageFormatArgs{
		ImageTemplate: variable.NewDefaultImageTemplate(),
	}

	return config
}
