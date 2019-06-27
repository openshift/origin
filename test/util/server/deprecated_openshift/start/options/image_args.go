package options

import (
	"github.com/openshift/openshift-controller-manager/pkg/cmd/imageformat"
)

// ImageFormatArgs is a struct that the command stores flag values into.
type ImageFormatArgs struct {
	// ImageTemplate is used in expanding parameterized container image references
	// from configuration or a file
	ImageTemplate imageformat.ImageTemplate
}

// NewDefaultImageFormatArgs returns the default image template
func NewDefaultImageFormatArgs() *ImageFormatArgs {
	config := &ImageFormatArgs{
		ImageTemplate: imageformat.NewDefaultImageTemplate(),
	}

	return config
}
