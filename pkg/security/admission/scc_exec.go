package admission

import (
	"io"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

func init() {
	admission.RegisterPlugin("SCCExecRestrictions", func(client client.Interface, config io.Reader) (admission.Interface, error) {
		execAdmitter := NewSCCExecRestrictions(client)
		execAdmitter.constraintAdmission.Run()
		return execAdmitter, nil
	})
}

// sccExecRestrictions is an implementation of admission.Interface which says no to a pod/exec on
// a pod that the user would not be allowed to create
type sccExecRestrictions struct {
	*admission.Handler
	constraintAdmission *constraint
	client              client.Interface
}

func (d *sccExecRestrictions) Admit(a admission.Attributes) (err error) {
	if a.GetOperation() != admission.Connect {
		return nil
	}
	if a.GetResource() != kapi.Resource("pods") {
		return nil
	}
	if a.GetSubresource() != "attach" && a.GetSubresource() != "exec" {
		return nil
	}

	pod, err := d.client.Pods(a.GetNamespace()).Get(a.GetName())
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	// create a synthentic admission attribute to check SCC admission status for this pod
	// clear the SA name, so that any permissions MUST be based on your user's power, not the SAs power.
	pod.Spec.ServiceAccountName = ""
	createAttributes := admission.NewAttributesRecord(pod, kapi.Kind("Pod"), a.GetNamespace(), a.GetName(), a.GetResource(), a.GetSubresource(), admission.Create, a.GetUserInfo())
	if err := d.constraintAdmission.Admit(createAttributes); err != nil {
		return admission.NewForbidden(a, err)
	}

	return nil
}

// NewSCCExecRestrictions creates a new admission controller that denies an exec operation on a privileged pod
func NewSCCExecRestrictions(client client.Interface) *sccExecRestrictions {
	return &sccExecRestrictions{
		Handler:             admission.NewHandler(admission.Connect),
		constraintAdmission: NewConstraint(client),
		client:              client,
	}
}
