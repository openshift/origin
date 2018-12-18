package resourceapply

import (
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextclientv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
)

// ApplyCustomResourceDefinition applies the required CustomResourceDefinition to the cluster.
func ApplyCustomResourceDefinition(client apiextclientv1beta1.CustomResourceDefinitionsGetter, recorder events.Recorder, required *apiextv1beta1.CustomResourceDefinition) (*apiextv1beta1.CustomResourceDefinition, bool, error) {
	existing, err := client.CustomResourceDefinitions().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.CustomResourceDefinitions().Create(required)
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureCustomResourceDefinition(modified, existing, *required)
	if !*modified {
		return existing, false, nil
	}

	actual, err := client.CustomResourceDefinitions().Update(existing)
	reportUpdateEvent(recorder, required, err)
	return actual, true, err
}
