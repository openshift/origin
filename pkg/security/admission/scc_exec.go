package admission

import (
	"io"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/controller/shared"
	"k8s.io/kubernetes/pkg/admission"
	kadmission "k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

func init() {
	admission.RegisterPlugin("SCCExecRestrictions", func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		execAdmitter := NewSCCExecRestrictions(client)
		return execAdmitter, nil
	})
}

var _ kadmission.Interface = &sccExecRestrictions{}
var _ = oadmission.WantsInformers(&sccExecRestrictions{})

// sccExecRestrictions is an implementation of admission.Interface which says no to a pod/exec on
// a pod that the user would not be allowed to create
type sccExecRestrictions struct {
	*admission.Handler
	constraintAdmission *constraint
	client              clientset.Interface
}

func (d *sccExecRestrictions) Admit(a admission.Attributes) (err error) {
	if a.GetOperation() != admission.Connect {
		return nil
	}
	if a.GetResource().GroupResource() != kapi.Resource("pods") {
		return nil
	}
	if a.GetSubresource() != "attach" && a.GetSubresource() != "exec" {
		return nil
	}

	pod, err := d.client.Core().Pods(a.GetNamespace()).Get(a.GetName())
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	// TODO, if we want to actually limit who can use which service account, then we'll need to add logic here to make sure that
	// we're allowed to use the SA the pod is using.  Otherwise, user-A creates pod and user-B (who can't use the SA) can exec into it.
	createAttributes := admission.NewAttributesRecord(pod, pod, kapi.Kind("Pod").WithVersion(""), a.GetNamespace(), a.GetName(), a.GetResource(), "", admission.Create, a.GetUserInfo())
	if err := d.constraintAdmission.Admit(createAttributes); err != nil {
		return admission.NewForbidden(a, err)
	}

	return nil
}

// NewSCCExecRestrictions creates a new admission controller that denies an exec operation on a privileged pod
func NewSCCExecRestrictions(client clientset.Interface) *sccExecRestrictions {
	return &sccExecRestrictions{
		Handler:             admission.NewHandler(admission.Connect),
		constraintAdmission: NewConstraint(client),
		client:              client,
	}
}

// SetInformers implements WantsInformers interface for sccExecRestrictions.
func (d *sccExecRestrictions) SetInformers(informers shared.InformerFactory) {
	d.constraintAdmission.sccLister = informers.SecurityContextConstraints().Lister()
}

// Validate defines actions to validate sccExecRestrictions
func (d *sccExecRestrictions) Validate() error {
	return d.constraintAdmission.Validate()
}
