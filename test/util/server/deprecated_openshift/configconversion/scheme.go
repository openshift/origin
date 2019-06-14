package configconversion

import (
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/clusterresourceoverride"
	clusterresourceoverridev1 "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/clusterresourceoverride/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration"
	runoncedurationv1 "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/autoscaling/apis/runonceduration/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/externalipranger"
	externaliprangerv1 "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/externalipranger/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/restrictedendpoints"
	restrictedendpointsv1 "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/network/apis/restrictedendpoints/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/route/apis/ingressadmission"
	ingressadmissionv1 "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/route/apis/ingressadmission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/apis/apiserver"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	auditv1alpha1 "k8s.io/apiserver/pkg/apis/audit/v1alpha1"
	auditv1beta1 "k8s.io/apiserver/pkg/apis/audit/v1beta1"

	imagepolicyapiv1 "github.com/openshift/origin/pkg/cmd/openshift-kube-apiserver/admission/imagepolicy/apis/imagepolicy/v1"
	"github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit"
	requestlimitv1 "github.com/openshift/origin/pkg/project/apiserver/admission/apis/requestlimit/v1"
	"github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints"
	podnodeconstraintsv1 "github.com/openshift/origin/pkg/scheduler/admission/apis/podnodeconstraints/v1"
	configapi "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config"
	configapiv1 "github.com/openshift/origin/test/util/server/deprecated_openshift/apis/config/v1"
)

var Scheme = runtime.NewScheme()

var Codecs = serializer.NewCodecFactory(Scheme)

var Codec = serializer.NewCodecFactory(Scheme).LegacyCodec(
	schema.GroupVersion{Group: "", Version: "v1"},
	schema.GroupVersion{Group: "network.openshift.io", Version: "v1"},
	schema.GroupVersion{Group: "autoscaling.openshift.io", Version: "v1"},
	schema.GroupVersion{Group: "image.openshift.io", Version: "v1"},
	schema.GroupVersion{Group: "scheduling.openshift.io", Version: "v1"},
	schema.GroupVersion{Group: "project.openshift.io", Version: "v1"},
	schema.GroupVersion{Group: "apiserver.k8s.io", Version: "v1alpha1"},
	schema.GroupVersion{Group: "audit.k8s.io", Version: "v1alpha1"},
	schema.GroupVersion{Group: "admission.config.openshift.io", Version: "v1"},
)

func init() {
	InstallLegacyInternal(Scheme)

	// yeah, nasty right. This is a fake path only until we move all integration to e2es.  The config we have is an intermediate that we only have because it's a
	// waste of time to actually configure all these tests. This injects the "correct" schema for conversions in that package to work.
	configapi.Scheme = Scheme
	configapi.Codecs = Codecs
}

func InstallLegacyInternal(scheme *runtime.Scheme) {
	utilruntime.Must(configapi.InstallLegacy(scheme))
	utilruntime.Must(configapiv1.InstallLegacy(scheme))

	// we additionally need to enable audit versions, since we embed the audit
	// policy file inside master-config.yaml
	utilruntime.Must(audit.AddToScheme(scheme))
	utilruntime.Must(auditv1alpha1.AddToScheme(scheme))
	utilruntime.Must(auditv1beta1.AddToScheme(scheme))
	utilruntime.Must(apiserver.AddToScheme(scheme))
	utilruntime.Must(apiserverv1alpha1.AddToScheme(scheme))
	utilruntime.Must(imagepolicyapiv1.Install(scheme))

	// add the other admission config types we have
	utilruntime.Must(requestlimit.Install(scheme))
	utilruntime.Must(requestlimitv1.Install(scheme))

	// add the other admission config types we have to the core group if they are legacy types
	utilruntime.Must(ingressadmission.Install(scheme))
	utilruntime.Must(ingressadmissionv1.Install(scheme))

	utilruntime.Must(clusterresourceoverride.Install(scheme))
	utilruntime.Must(clusterresourceoverridev1.Install(scheme))

	utilruntime.Must(runonceduration.Install(scheme))
	utilruntime.Must(runoncedurationv1.Install(scheme))

	utilruntime.Must(podnodeconstraints.Install(scheme))
	utilruntime.Must(podnodeconstraintsv1.Install(scheme))

	utilruntime.Must(restrictedendpoints.Install(scheme))
	utilruntime.Must(restrictedendpointsv1.Install(scheme))

	utilruntime.Must(externalipranger.Install(scheme))
	utilruntime.Must(externaliprangerv1.Install(scheme))
}
