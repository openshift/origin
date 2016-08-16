// This plugin supplements upstream ResourceQuota admission plugin.
// It takes care of OpenShift specific resources that may be abusing resource quota limits.

package resourcequota

import (
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

const PluginName = "openshift.io/OriginResourceQuota"

func init() {
	admission.RegisterPlugin(PluginName,
		func(kClient clientset.Interface, config io.Reader) (admission.Interface, error) {
			return NewOriginResourceQuota(kClient), nil
		})
}

// originQuotaAdmission implements an admission controller that can enforce quota constraints on images and image
// streams
type originQuotaAdmission struct {
	*admission.Handler
	kQuotaAdmission admission.Interface
	// must be able to read/write ResourceQuota
	kClient clientset.Interface
}

var _ = oadmission.WantsOriginQuotaRegistry(&originQuotaAdmission{})
var _ = oadmission.Validator(&originQuotaAdmission{})

// NewOriginResourceQuota creates a new OriginResourceQuota admission plugin that takes care of admission of
// origin resources abusing resource quota.
func NewOriginResourceQuota(kClient clientset.Interface) admission.Interface {
	// defer an initialization of upstream controller until os client is set
	return &originQuotaAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
		kClient: kClient,
	}
}

func (a *originQuotaAdmission) Admit(as admission.Attributes) error {
	return a.kQuotaAdmission.Admit(as)
}

func (a *originQuotaAdmission) SetOriginQuotaRegistry(registry kquota.Registry) {
	// TODO: Make the number of evaluators configurable?
	quotaAdmission, err := resourcequota.NewResourceQuota(a.kClient, registry, 5, wait.NeverStop)
	if err != nil {
		glog.Fatalf("failed to initialize %s plugin: %v", PluginName, err)
	}
	a.kQuotaAdmission = quotaAdmission
}

func (a *originQuotaAdmission) Validate() error {
	if a.kQuotaAdmission == nil {
		return fmt.Errorf("%s requires an origin quota registry", PluginName)
	}
	return nil
}
