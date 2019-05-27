package install

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/openshift-apiserver/admission/externalipranger/apis/externalipranger"
	"github.com/openshift/openshift-apiserver/admission/externalipranger/apis/externalipranger/v1"
)

func InstallInternal(scheme *runtime.Scheme) {
	utilruntime.Must(externalipranger.Install(scheme))
	utilruntime.Must(v1.Install(scheme))
}
