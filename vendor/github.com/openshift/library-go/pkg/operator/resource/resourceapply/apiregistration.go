package resourceapply

import (
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1client "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"

	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyAPIService merges objectmeta and requires apiservice coordinates.  It does not touch CA bundles, which should be managed via service CA controller.
func ApplyAPIService(client apiregistrationv1client.APIServicesGetter, required *apiregistrationv1.APIService) (*apiregistrationv1.APIService, bool, error) {
	existing, err := client.APIServices().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.APIServices().Create(required)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)
	serviceSame := equality.Semantic.DeepEqual(existing.Spec.Service, required.Spec.Service)
	prioritySame := existing.Spec.VersionPriority == required.Spec.VersionPriority && existing.Spec.GroupPriorityMinimum == required.Spec.GroupPriorityMinimum
	insecureSame := existing.Spec.InsecureSkipTLSVerify == required.Spec.InsecureSkipTLSVerify
	// there was no change to metadata, the service and priorities were right
	if !*modified && serviceSame && prioritySame && insecureSame {
		return existing, false, nil
	}

	existing.Spec = required.Spec

	actual, err := client.APIServices().Update(existing)
	return actual, true, err
}
