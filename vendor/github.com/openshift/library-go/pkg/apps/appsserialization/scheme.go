package appsserialization

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	appsv1 "github.com/openshift/api/apps/v1"
)

var (
	// for decoding, we want to be tolerant of groupified and non-groupified
	annotationDecodingScheme = runtime.NewScheme()
	annotationDecoder        runtime.Decoder

	// for encoding, we want to be strict on groupified
	annotationEncodingScheme = runtime.NewScheme()
	annotationEncoder        runtime.Encoder
)

func init() {
	utilruntime.Must(appsv1.Install(annotationDecodingScheme))
	utilruntime.Must(appsv1.DeprecatedInstallWithoutGroup(annotationDecodingScheme))
	annotationDecoderCodecFactory := serializer.NewCodecFactory(annotationDecodingScheme)
	annotationDecoder = annotationDecoderCodecFactory.UniversalDecoder(appsv1.GroupVersion)

	utilruntime.Must(appsv1.Install(annotationEncodingScheme))
	annotationEncoderCodecFactory := serializer.NewCodecFactory(annotationEncodingScheme)
	annotationEncoder = annotationEncoderCodecFactory.LegacyCodec(appsv1.GroupVersion)
}
