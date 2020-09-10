package resourceapply

import (
	"context"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextclientv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apiextclientv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// ApplyCustomResourceDefinitionV1Beta1 applies the required CustomResourceDefinition to the cluster.
func ApplyCustomResourceDefinitionV1Beta1(client apiextclientv1beta1.CustomResourceDefinitionsGetter, recorder events.Recorder, required *apiextv1beta1.CustomResourceDefinition) (*apiextv1beta1.CustomResourceDefinition, bool, error) {
	existing, err := client.CustomResourceDefinitions().Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.CustomResourceDefinitions().Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()
	resourcemerge.EnsureCustomResourceDefinitionV1Beta1(modified, existingCopy, *required)
	if !*modified {
		return existing, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("CustomResourceDefinition %q changes: %s", existing.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.CustomResourceDefinitions().Update(context.TODO(), existingCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)

	return actual, true, err
}

// ApplyCustomResourceDefinitionV1 applies the required CustomResourceDefinition to the cluster.
func ApplyCustomResourceDefinitionV1(client apiextclientv1.CustomResourceDefinitionsGetter, recorder events.Recorder, required *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, bool, error) {
	existing, err := client.CustomResourceDefinitions().Get(context.TODO(), required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		actual, err := client.CustomResourceDefinitions().Create(context.TODO(), required, metav1.CreateOptions{})
		reportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := resourcemerge.BoolPtr(false)
	existingCopy := existing.DeepCopy()
	resourcemerge.EnsureCustomResourceDefinitionV1(modified, existingCopy, *required)
	if !*modified {
		return existing, false, nil
	}

	if klog.V(4).Enabled() {
		klog.Infof("CustomResourceDefinition %q changes: %s", existing.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.CustomResourceDefinitions().Update(context.TODO(), existingCopy, metav1.UpdateOptions{})
	reportUpdateEvent(recorder, required, err)

	return actual, true, err
}
