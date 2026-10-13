package resourceapply

import (
	"context"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextclientv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// ApplyCustomResourceDefinitionV1 applies the required CustomResourceDefinition to the cluster.
func ApplyCustomResourceDefinitionV1(ctx context.Context, client apiextclientv1.CustomResourceDefinitionsGetter, recorder events.Recorder, required *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, bool, error) {
	existing, err := client.CustomResourceDefinitions().Get(ctx, required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		requiredCopy := required.DeepCopy()
		actual, err := client.CustomResourceDefinitions().Create(
			ctx, resourcemerge.WithCleanLabelsAndAnnotations(requiredCopy).(*apiextensionsv1.CustomResourceDefinition), metav1.CreateOptions{})
		resourcehelper.ReportCreateEvent(recorder, required, err)
		return actual, true, err
	}
	if err != nil {
		return nil, false, err
	}

	modified := false
	existingCopy := existing.DeepCopy()
	resourcemerge.EnsureCustomResourceDefinitionV1(&modified, existingCopy, *required)
	if !modified {
		return existing, false, nil
	}

	if klog.V(2).Enabled() {
		klog.Infof("CustomResourceDefinition %q changes: %s", existing.Name, JSONPatchNoError(existing, existingCopy))
	}

	actual, err := client.CustomResourceDefinitions().Update(ctx, existingCopy, metav1.UpdateOptions{})
	resourcehelper.ReportUpdateEvent(recorder, required, err)

	return actual, true, err
}

func DeleteCustomResourceDefinitionV1(ctx context.Context, client apiextclientv1.CustomResourceDefinitionsGetter, recorder events.Recorder, required *apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, bool, error) {
	err := client.CustomResourceDefinitions().Delete(ctx, required.Name, metav1.DeleteOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	resourcehelper.ReportDeleteEvent(recorder, required, err)
	return nil, true, nil
}
