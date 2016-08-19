package ownerref

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api/meta"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

func init() {
	admission.RegisterPlugin("openshift.io/OwnerReference",
		func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
			return NewOwnerReferenceBlocker()
		})
}

// ownerReferenceAdmission implements an admission controller that stops anyone from setting owner references
// TODO enforce this check based on the level of access allowed.  If you can delete the object, you can add an
// owner reference.  We need to make sure the upstream controller works before allowing this.
type ownerReferenceAdmission struct {
	*admission.Handler
}

func NewOwnerReferenceBlocker() (admission.Interface, error) {
	return &ownerReferenceAdmission{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// Admit makes admission decisions while enforcing ownerReference
func (q *ownerReferenceAdmission) Admit(a admission.Attributes) (err error) {
	metadata, err := meta.Accessor(a.GetObject())
	if err != nil {
		// if we don't have object meta, we don't have fields we're trying to control
		return nil
	}

	// TODO if we have an old object, only consider new owner references and finalizers
	// this is critical when doing an actual authz check.

	if ownerRefs := metadata.GetOwnerReferences(); len(ownerRefs) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("ownerReferences are disabled: %v", ownerRefs))
	}
	if finalizers := metadata.GetFinalizers(); len(finalizers) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("finalizers are disabled: %v", finalizers))
	}

	return nil
}
