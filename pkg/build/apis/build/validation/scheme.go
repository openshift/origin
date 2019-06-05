package validation

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
	buildv1helpers "github.com/openshift/origin/pkg/build/apis/build/v1"
)

var (
	// encoder always encodes to groupfied.
	encoder runtime.Encoder
)

func init() {

	encoderScheme := runtime.NewScheme()
	utilruntime.Must(buildv1helpers.Install(encoderScheme))
	annotationEncoderCodecFactory := serializer.NewCodecFactory(encoderScheme)
	encoder = annotationEncoderCodecFactory.LegacyCodec(buildv1.GroupVersion)
}
