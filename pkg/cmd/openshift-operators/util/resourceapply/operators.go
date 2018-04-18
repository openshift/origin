package resourceapply

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	webconsolev1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/webconsole/v1alpha1"
	webconsoleclientv1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/generated/clientset/versioned/typed/webconsole/v1alpha1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
)

func ApplyWebConsoleOperatorConfig(client webconsoleclientv1alpha1.OpenShiftWebConsoleConfigsGetter, required *webconsolev1alpha1.OpenShiftWebConsoleConfig) (bool, error) {
	existing, err := client.OpenShiftWebConsoleConfigs().Get(required.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err := client.OpenShiftWebConsoleConfigs().Create(required)
		return true, err
	}
	if err != nil {
		return false, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureWebConsoleOperatorConfig(modified, existing, *required)
	if !*modified {
		return false, nil
	}
	_, err = client.OpenShiftWebConsoleConfigs().Update(existing)
	return true, err
}
