package buildscheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildv1helpers "github.com/openshift/origin/pkg/build/apis/build/v1"
)

var (
	// Decoder understands groupified and non-groupfied.  It deals in internals for now, but will be updated later
	Decoder runtime.Decoder

	// EncoderScheme can identify types for serialization. We use this for the event recorder and other things that need to
	// identify external kinds.
	EncoderScheme = runtime.NewScheme()
	// Encoder always encodes to groupfied.
	Encoder runtime.Encoder

	// provides a way to convert between internal and external.  Please don't used this to serialize and deserialize
	// Use this for places where you have to convert to some kind of a helper.  It happens in apiserver flows where you have
	// internal objects available
	InternalExternalScheme = runtime.NewScheme()
)

func init() {
	annotationDecodingScheme := runtime.NewScheme()
	utilruntime.Must(buildv1.Install(annotationDecodingScheme))
	utilruntime.Must(buildv1helpers.AddToScheme(annotationDecodingScheme))
	utilruntime.Must(buildv1.DeprecatedInstallWithoutGroup(annotationDecodingScheme))
	utilruntime.Must(buildv1helpers.AddToSchemeInCoreGroup(annotationDecodingScheme))
	// TODO eventually we shouldn't deal in internal versions, but for now decode into one.
	utilruntime.Must(buildapi.AddToScheme(annotationDecodingScheme))
	utilruntime.Must(buildapi.AddToSchemeInCoreGroup(annotationDecodingScheme))
	annotationDecoderCodecFactory := serializer.NewCodecFactory(annotationDecodingScheme)
	Decoder = annotationDecoderCodecFactory.UniversalDecoder(buildapi.SchemeGroupVersion)

	utilruntime.Must(buildv1.Install(EncoderScheme))
	// TODO eventually we shouldn't deal in internal versions, but for now encode from one.
	utilruntime.Must(buildv1helpers.AddToScheme(EncoderScheme))
	utilruntime.Must(buildapi.AddToScheme(EncoderScheme))
	annotationEncoderCodecFactory := serializer.NewCodecFactory(EncoderScheme)
	Encoder = annotationEncoderCodecFactory.LegacyCodec(buildv1.SchemeGroupVersion)

	utilruntime.Must(buildv1.Install(InternalExternalScheme))
	utilruntime.Must(buildv1helpers.AddToScheme(InternalExternalScheme))
	utilruntime.Must(buildapi.AddToScheme(InternalExternalScheme))
}
