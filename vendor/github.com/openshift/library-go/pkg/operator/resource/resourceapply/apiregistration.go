package resourceapply

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyAPIService merges objectmeta and requires apiservice coordinates.  It does not touch CA bundles, which should be managed via service CA controller.
func ApplyAPIService(ctx context.Context, client apiregistrationv1client.APIServicesGetter, recorder events.Recorder, required *apiregistrationv1.APIService) (*apiregistrationv1.APIService, bool, error) {
	existing, err := client.APIServices().Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.APIServices().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*apiregistrationv1.APIService), metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := false
	existingCopy := existing.DeepCopy()

	resourcemerge.EnsureObjectMeta(&modified, &existingCopy.ObjectMeta, required.ObjectMeta)
	serviceSame := equality.Semantic.DeepEqual(existingCopy.Spec.Service, required.Spec.Service)
	prioritySame := existingCopy.Spec.VersionPriority == required.Spec.VersionPriority && existingCopy.Spec.GroupPriorityMinimum == required.Spec.GroupPriorityMinimum
	insecureSame := existingCopy.Spec.InsecureSkipTLSVerify == required.Spec.InsecureSkipTLSVerify
	// there was no change to metadata, the service and priorities were right
	if !modified && serviceSame && prioritySame && insecureSame {
		return existingCopy, false, nil
	}

	existingCopy.Spec = required.Spec

	if klog.V(2).Enabled() {
		klog.Infof("APIService %q changes: %s", existing.Name, JSONPatchNoError(existing, existingCopy))
	}
	actual, err := client.APIServices().Update(ctx, existingCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}
