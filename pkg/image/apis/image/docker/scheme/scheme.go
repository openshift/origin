package scheme

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/api/image/dockerpre012"
	"github.com/openshift/origin/pkg/image/apis/image/docker"
	internaldockerpre012 "github.com/openshift/origin/pkg/image/apis/image/dockerpre012"
)

var Scheme = runtime.NewScheme()
var Codecs = serializer.NewCodecFactory(Scheme)
var ParameterCodec = runtime.NewParameterCodec(Scheme)

func init() {
	AddToScheme(Scheme)
}

func AddToScheme(scheme *runtime.Scheme) {
	docker.AddToScheme(scheme)
	docker.AddToSchemeInCoreGroup(scheme)
	docker10.AddToScheme(scheme)
	docker10.AddToSchemeInCoreGroup(scheme)
	dockerpre012.AddToScheme(scheme)
	dockerpre012.AddToSchemeInCoreGroup(scheme)
	internaldockerpre012.AddToScheme(scheme)
	internaldockerpre012.AddToSchemeInCoreGroup(scheme)
}
