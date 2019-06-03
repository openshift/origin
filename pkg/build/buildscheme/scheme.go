package buildscheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
)

var (
	// Decoder understands groupified and non-groupfied.  It deals in internals for now, but will be updated later
	Decoder runtime.Decoder

	// EncoderScheme can identify types for serialization. We use this for the event recorder and other things that need to
	// identify external kinds.
	EncoderScheme = runtime.NewScheme()
	// Encoder always encodes to groupfied.
	Encoder runtime.Encoder
)

func init() {
	annotationDecodingScheme := runtime.NewScheme()
	utilruntime.Must(buildv1.Install(annotationDecodingScheme))
	utilruntime.Must(buildv1.DeprecatedInstallWithoutGroup(annotationDecodingScheme))
	annotationDecoderCodecFactory := serializer.NewCodecFactory(annotationDecodingScheme)
	Decoder = annotationDecoderCodecFactory.UniversalDecoder(buildv1.GroupVersion)

	utilruntime.Must(buildv1.Install(EncoderScheme))
	annotationEncoderCodecFactory := serializer.NewCodecFactory(EncoderScheme)
	Encoder = annotationEncoderCodecFactory.LegacyCodec(buildv1.GroupVersion)
}
