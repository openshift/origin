package resourceapply

import (
	"github.com/golang/glog"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storageclientv1 "k8s.io/client-go/kubernetes/typed/storage/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyStorageClass merges objectmeta, tries to write everything else
func ApplyStorageClass(client storageclientv1.StorageClassesGetter, recorder events.Recorder, required *storagev1.StorageClass) (*storagev1.StorageClass, bool,
	error) {
	existing, err := client.StorageClasses().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.StorageClasses().Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	contentSame := equality.Semantic.DeepEqual(existingCopy, required)
	if contentSame && !*modified {
		return existingCopy, false, nil
	}

	objectMeta := existingCopy.ObjectMeta.DeepCopy()
	existingCopy = required.DeepCopy()
	existingCopy.ObjectMeta = *objectMeta

	if glog.V(4) {
		glog.Infof("StorageClass %q changes: %v", required.Name, JSONPatch(existing, existingCopy))
	}

	// TODO if provisioner, parameters, reclaimpolicy, or volumebindingmode are different, update will fail so delete and recreate
	actual, err := client.StorageClasses().Update(existingCopy)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}
