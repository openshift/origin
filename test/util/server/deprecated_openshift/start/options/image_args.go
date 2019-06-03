package options

import (
	"github.com/openshift/origin/pkg/cmd/openshift-controller-manager/imageformat"
)

// ImageFormatArgs is a struct that the command stores flag values into.
type ImageFormatArgs struct {
	// ImageTemplate is used in expanding parameterized Docker image references
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
