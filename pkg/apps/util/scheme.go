package util

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsv1helpers "github.com/openshift/origin/pkg/apps/apis/apps/v1"
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
	// TODO eventually we shouldn't deal in internal versions, but for now decode into one.
	utilruntime.Must(appsv1helpers.AddToScheme(annotationDecodingScheme))
	utilruntime.Must(appsv1helpers.AddToSchemeInCoreGroup(annotationDecodingScheme))
	utilruntime.Must(coreapi.AddToScheme(annotationDecodingScheme))
	utilruntime.Must(appsapi.AddToScheme(annotationDecodingScheme))
	utilruntime.Must(appsapi.AddToSchemeInCoreGroup(annotationDecodingScheme))
	annotationDecoderCodecFactory := serializer.NewCodecFactory(annotationDecodingScheme)
	annotationDecoder = annotationDecoderCodecFactory.UniversalDecoder(appsapi.SchemeGroupVersion)

	utilruntime.Must(appsv1.Install(annotationEncodingScheme))
	// TODO eventually we shouldn't deal in internal versions, but for now decode into one.
	utilruntime.Must(appsv1helpers.AddToScheme(annotationEncodingScheme))
	utilruntime.Must(coreapi.AddToScheme(annotationEncodingScheme))
	utilruntime.Must(appsapi.AddToScheme(annotationEncodingScheme))
	annotationEncoderCodecFactory := serializer.NewCodecFactory(annotationEncodingScheme)
	annotationEncoder = annotationEncoderCodecFactory.LegacyCodec(appsv1.SchemeGroupVersion)
}
