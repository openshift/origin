// This plugin supplements upstream ResourceQuota admission plugin.
// It takes care of OpenShift specific resources that may be abusing resource quota limits.

package resourcequota

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"
	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

const PluginName = "openshift.io/OriginResourceQuota"

func init() {
	admission.RegisterPlugin(PluginName,
		func(config io.Reader) (admission.Interface, error) {
			return NewOriginResourceQuota(), nil
		})
}

// originQuotaAdmission implements an admission controller that can enforce quota constraints on images and image
// streams
type originQuotaAdmission struct {
	*admission.Handler
	quotaAdmission admission.Interface
	// must be able to read/write ResourceQuota
	kclient kclientset.Interface
}

var _ = oadmission.WantsOriginQuotaRegistry(&originQuotaAdmission{})
var _ = kadmission.WantsInternalKubeClientSet(&originQuotaAdmission{})
var _ = kadmission.WantsInternalKubeInformerFactory(&originQuotaAdmission{})

// NewOriginResourceQuota creates a new OriginResourceQuota admission plugin that takes care of admission of
// origin resources abusing resource quota.
func NewOriginResourceQuota() admission.Interface {
	// defer an initialization of upstream controller until os client is set
	return &originQuotaAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

func (a *originQuotaAdmission) Admit(as admission.Attributes) error {
	return a.quotaAdmission.Admit(as)
}

func (a *originQuotaAdmission) SetOriginQuotaRegistry(registry kquota.Registry) {
	// TODO: Make the number of evaluators configurable?
	quotaAdmission, err := resourcequota.NewResourceQuota(registry, &resourcequotaapi.Configuration{}, 5, wait.NeverStop)
	if err != nil {
		glog.Fatalf("failed to initialize %s plugin: %v", PluginName, err)
	}
	a.quotaAdmission = quotaAdmission
	if a.kclient != nil {
		a.quotaAdmission.(kadmission.WantsInternalKubeClientSet).SetInternalKubeClientSet(a.kclient)
	}
}

func (a *originQuotaAdmission) SetInternalKubeClientSet(c kclientset.Interface) {
	a.kclient = c
	if a.quotaAdmission != nil {
		a.quotaAdmission.(kadmission.WantsInternalKubeClientSet).SetInternalKubeClientSet(c)
	}
}

func (a *originQuotaAdmission) SetInternalKubeInformerFactory(f kinternalinformers.SharedInformerFactory) {
	if a.quotaAdmission != nil {
		a.quotaAdmission.(kadmission.WantsInternalKubeInformerFactory).SetInternalKubeInformerFactory(f)
	}
}

func (a *originQuotaAdmission) Validate() error {
	if a.quotaAdmission == nil {
		return fmt.Errorf("%s requires an origin quota registry", PluginName)
	}
	return nil
}
