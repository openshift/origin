package controller

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1helpers"
)

func (c WebConsoleOperator) sync10_0(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) (*webconsolev1.OpenShiftWebConsoleConfig, []error) {
	versionAvailability := webconsolev1.WebConsoleVersionAvailablity{
		Version: "3.10.0",
	}

	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyService(c.corev1Client, resourceread.ReadServiceOrDie([]byte(service10Yaml))); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "webconsole"}}); err != nil {
		errors = append(errors, err)
	}

	// we need to make a configmap here
	if _, err := c.ensureWebConsoleConfigMap10(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	actualDeployment, _, err := c.ensureWebConsoleDeployment10(operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}
	if actualDeployment != nil {
		versionAvailability.UpdatedReplicas = actualDeployment.Status.UpdatedReplicas
		versionAvailability.AvailableReplicas = actualDeployment.Status.AvailableReplicas
		versionAvailability.ReadyReplicas = actualDeployment.Status.ReadyReplicas
	}

	v1helpers.SetErrors(&versionAvailability, errors...)
	v1helpers.SetVersionAvailablity(&operatorConfig.Status.VersionAvailability, versionAvailability)
	if operatorConfig.Spec.Version == "3.10.0" && versionAvailability.AvailableReplicas > 0 {
		operatorConfig.Status.Version = "3.10.0"
	}

	return operatorConfig, errors
}

func (c WebConsoleOperator) ensureWebConsoleConfigMap10(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	requiredConfig, err := ensureWebConsoleConfig(options)
	if err != nil {
		return false, err
	}

	newWebConsoleConfig, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(webconsoleconfigv1.SchemeGroupVersion), requiredConfig)
	if err != nil {
		return false, err
	}
	requiredConfigMap := resourceread.ReadConfigMapOrDie([]byte(configMap10Yaml))
	requiredConfigMap.Data[configConfigMap10Key] = string(newWebConsoleConfig)

	return resourceapply.ApplyConfigMap(c.corev1Client, requiredConfigMap)
}

func (c WebConsoleOperator) ensureWebConsoleDeployment10(options webconsolev1.OpenShiftWebConsoleConfigSpec) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentOrDie([]byte(deployment10Yaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.LogLevel))
	required.Spec.Replicas = &options.Replicas
	required.Spec.Template.Spec.NodeSelector = options.NodeSelector

	return resourceapply.ApplyDeployment(c.appsv1Client, required)
}

func is10_0Version(version string) bool {
	return version == "3.10.0" || version == "3.10"
}
