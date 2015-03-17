package start

import (
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/util/variable"
)

// OriginMasterArgs is a struct that the command stores flag values into.
type ImageFormatArgs struct {
	ImageTemplate variable.ImageTemplate
}

func BindImageFormatArgs(args *ImageFormatArgs, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&args.ImageTemplate.Format, "images", args.ImageTemplate.Format, "When fetching images used by the cluster for important components, use this format on both master and nodes. The latest release will be used by default.")
	flags.BoolVar(&args.ImageTemplate.Latest, "latest-images", args.ImageTemplate.Latest, "If true, attempt to use the latest images for the cluster instead of the latest release.")
}

func NewDefaultImageFormatArgs() *ImageFormatArgs {
	config := &ImageFormatArgs{
		ImageTemplate: variable.NewDefaultImageTemplate(),
	}

	return config
}
