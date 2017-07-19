package admission

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
)

// IsOnlyMutatingGCFields checks finalizers and ownerrefs which GC manipulates
// and indicates that only those fields are changing
func IsOnlyMutatingGCFields(obj, old runtime.Object) bool {
	// make a copy of the newObj so that we can stomp for comparison
	copied, err := kapi.Scheme.Copy(obj)
	if err != nil {
		// if we couldn't copy, don't fail, just make it do the check
		return false
	}
	copiedMeta, err := meta.Accessor(copied)
	if err != nil {
		return false
	}
	oldMeta, err := meta.Accessor(old)
	if err != nil {
		return false
	}
	copiedMeta.SetOwnerReferences(oldMeta.GetOwnerReferences())
	copiedMeta.SetFinalizers(oldMeta.GetFinalizers())
	copiedMeta.SetSelfLink(oldMeta.GetSelfLink())

	return kapihelper.Semantic.DeepEqual(copied, old)
}
