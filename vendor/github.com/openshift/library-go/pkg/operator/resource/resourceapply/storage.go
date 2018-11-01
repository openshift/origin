package resourceapply

import (
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storageclientv1 "k8s.io/client-go/kubernetes/typed/storage/v1"

	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyStorageClass merges objectmeta, tries to write everything else
func ApplyStorageClass(client storageclientv1.StorageClassesGetter, required *storagev1.StorageClass) (*storagev1.StorageClass, bool, error) {
	existing, err := client.StorageClasses().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.StorageClasses().Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existing, required)
	if contentSame && !*modified {
		return existing, false, nil
	}

	objectMeta := existing.ObjectMeta.DeepCopy()
	existing = required.DeepCopy()
	existing.ObjectMeta = *objectMeta

	// TODO if provisioner, parameters, reclaimpolicy, or volumebindingmode are different, update will fail so delete and recreate
	actual, err := client.StorageClasses().Update(existing)
	return actual, true, err
}
