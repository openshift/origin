package resourceapply

import (
	apiserverv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/apis/apiserver/v1"
	apiserverclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned/typed/apiserver/v1"
	controllerv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/apis/controller/v1"
	controllerclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned/typed/controller/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	webconsoleclientv1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned/typed/webconsole/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ApplyControllerOperatorConfig(client controllerclientv1.OpenShiftControllerConfigsGetter, required *controllerv1.OpenShiftControllerConfig) (bool, error) {
	existing, err := client.OpenShiftControllerConfigs().Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		_, err := client.OpenShiftControllerConfigs().Create(required)
		return true, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureControllerOperatorConfig(modified, existing, *required)
	if !*modified {
		return false, nil
	}
	_, err = client.OpenShiftControllerConfigs().Update(existing)
	return true, err
}

func ApplyAPIServerOperatorConfig(client apiserverclientv1.OpenShiftAPIServerConfigsGetter, required *apiserverv1.OpenShiftAPIServerConfig) (bool, error) {
	existing, err := client.OpenShiftAPIServerConfigs().Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		_, err := client.OpenShiftAPIServerConfigs().Create(required)
		return true, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureAPIServerOperatorConfig(modified, existing, *required)
	if !*modified {
		return false, nil
	}
	_, err = client.OpenShiftAPIServerConfigs().Update(existing)
	return true, err
}

func ApplyWebConsoleOperatorConfig(client webconsoleclientv1.OpenShiftWebConsoleConfigsGetter, required *webconsolev1.OpenShiftWebConsoleConfig) (bool, error) {
	existing, err := client.OpenShiftWebConsoleConfigs().Get(required.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		_, err := client.OpenShiftWebConsoleConfigs().Create(required)
		return true, err
	}

	modified := resourcemerge.BoolPtr(false)
	resourcemerge.EnsureWebConsoleOperatorConfig(modified, existing, *required)
	if !*modified {
		return false, nil
	}
	_, err = client.OpenShiftWebConsoleConfigs().Update(existing)
	return true, err
}
