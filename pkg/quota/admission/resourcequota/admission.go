// This plugin supplements upstream ResourceQuota admission plugin.
// It takes care of OpenShift specific resources that may be abusing resource quota limits.

package admission

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	osclient "github.com/openshift/origin/pkg/client"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/quota"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

const pluginName = "OriginResourceQuota"

func init() {
	admission.RegisterPlugin(pluginName,
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
	kClient   clientset.Interface
	osClient  osclient.Interface
	rcFactory quotautil.InternalRegistryClientFactory
}

var _ = oadmission.WantsOpenshiftClient(&originQuotaAdmission{})
var _ = oadmission.WantsInternalRegistryClientFactory(&originQuotaAdmission{})
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

func (a *originQuotaAdmission) SetOpenshiftClient(osClient osclient.Interface) {
	a.osClient = osClient
	a.initializeRegistry()
}

func (a *originQuotaAdmission) SetInternalRegistryClientFactory(rcFactory quotautil.InternalRegistryClientFactory) {
	a.rcFactory = rcFactory
	a.initializeRegistry()
}

func (a *originQuotaAdmission) Validate() error {
	if a.kQuotaAdmission == nil {
		return errors.New(fmt.Sprintf("%s requires an openshift client and registry client factory", pluginName))
	}
	return nil
}

func (a *originQuotaAdmission) initializeRegistry() {
	if a.osClient != nil && a.rcFactory != nil {
		registry := quota.NewRegistry(a.osClient, a.rcFactory)
		quotaAdmission, err := resourcequota.NewResourceQuota(a.kClient, registry)
		if err != nil {
			glog.Fatalf("failed to initialize %s plugin: %v", pluginName, err)
		}
		a.kQuotaAdmission = quotaAdmission
	}
}
