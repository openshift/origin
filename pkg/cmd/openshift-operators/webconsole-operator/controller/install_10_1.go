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

// between 3.10.0 and 3.10.1, we want to fix some naming mistakes.  During this time we need to ensure the
// 3.10.0 resources are correct and we need to spin up 3.10.1 *before* removing the old
func (c WebConsoleOperator) migrate10_0_to_10_1(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) (*webconsolev1.OpenShiftWebConsoleConfig, []error) {
	versionAvailability := webconsolev1.WebConsoleVersionAvailablity{
		Version: "3.10.1",
	}

	errors := []error{}
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "web-console"}}); err != nil {
		errors = append(errors, err)
	}

	// keep 3.10.0 up to date in case we have to do things like change the config
	// we track the operatorConfig to make sure we have the resource version for updates
	var oldSyncErrors []error
	operatorConfig, oldSyncErrors = c.sync10_0(operatorConfig)
	errors = append(errors, oldSyncErrors...)

	// create the 3.10.1 resources
	_, err := c.ensureWebConsoleConfigMap10_1(operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}
	actualDeployment, _, err := c.ensureWebConsoleDeployment10_1(operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}
	if actualDeployment != nil {
		versionAvailability.UpdatedReplicas = actualDeployment.Status.UpdatedReplicas
		versionAvailability.AvailableReplicas = actualDeployment.Status.AvailableReplicas
		versionAvailability.ReadyReplicas = actualDeployment.Status.ReadyReplicas
	}

	deployment10_1IsReady := actualDeployment.Status.ReadyReplicas > 0
	if !deployment10_1IsReady || len(errors) > 0 {
		v1helpers.SetErrors(&versionAvailability, errors...)
		v1helpers.SetVersionAvailablity(&operatorConfig.Status.VersionAvailability, versionAvailability)
		return operatorConfig, errors
	}

	// after the new deployment has ready pods, we can swap the service and remove the old resources
	// TODO check the apiserver version to make this choice
	if _, err := resourceapply.ApplyService(c.corev1Client, resourceread.ReadServiceOrDie([]byte(service10_1_pre10_2APIServerYaml))); err != nil {
		errors = append(errors, err)
	}

	// if we have errors at this point, we cannot remove the 3.10.0 resources
	if len(errors) > 0 {
		v1helpers.SetErrors(&versionAvailability, errors...)
		v1helpers.SetVersionAvailablity(&operatorConfig.Status.VersionAvailability, versionAvailability)
		return operatorConfig, errors
	}

	if err := c.appsv1Client.Deployments("openshift-web-console").Delete("webconsole", nil); err != nil {
		errors = append(errors, err)
	}

	if err := c.corev1Client.ConfigMaps("openshift-web-console").Delete("webconsole-config", nil); err != nil {
		errors = append(errors, err)
	}

	// set the status
	v1helpers.SetErrors(&versionAvailability, errors...)
	v1helpers.SetVersionAvailablity(&operatorConfig.Status.VersionAvailability, versionAvailability)
	v1helpers.RemoveAvailability(&operatorConfig.Status.VersionAvailability, "3.10.0")
	if operatorConfig.Spec.Version == "3.10.1" && versionAvailability.AvailableReplicas > 0 {
		operatorConfig.Status.Version = "3.10.1"
	}

	return operatorConfig, errors
}

func (c WebConsoleOperator) sync10_1(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) (*webconsolev1.OpenShiftWebConsoleConfig, []error) {
	versionAvailability := webconsolev1.WebConsoleVersionAvailablity{
		Version: "3.10.1",
	}

	errors := []error{}
	// TODO the configmap and secret changes for daemonset should actually be a newly created configmap and then a subsequent daemonset update
	// TODO this requires us to be able to detect that the changes have not worked well and trigger an effective rollback to previous config
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	// TODO check the apiserver version to make this choice
	if _, err := resourceapply.ApplyService(c.corev1Client, resourceread.ReadServiceOrDie([]byte(service10_1_pre10_2APIServerYaml))); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "web-console"}}); err != nil {
		errors = append(errors, err)
	}

	// we need to make a configmap here
	if _, err := c.ensureWebConsoleConfigMap10_1(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	actualDeployment, _, err := c.ensureWebConsoleDeployment10_1(operatorConfig.Spec)
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
	if operatorConfig.Spec.Version == "3.10.1" && versionAvailability.AvailableReplicas > 0 {
		operatorConfig.Status.Version = "3.10.1"
	}

	return operatorConfig, errors
}

func (c WebConsoleOperator) ensureWebConsoleConfigMap10_1(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	requiredConfig, err := ensureWebConsoleConfig(options)
	if err != nil {
		return false, err
	}

	newWebConsoleConfig, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(webconsoleconfigv1.SchemeGroupVersion), requiredConfig)
	if err != nil {
		return false, err
	}
	requiredConfigMap := resourceread.ReadConfigMapOrDie([]byte(configMap10_1Yaml))
	requiredConfigMap.Data[configConfigMap10_1Key] = string(newWebConsoleConfig)

	return resourceapply.ApplyConfigMap(c.corev1Client, requiredConfigMap)
}

func (c WebConsoleOperator) ensureWebConsoleDeployment10_1(options webconsolev1.OpenShiftWebConsoleConfigSpec) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentOrDie([]byte(deployment10_1Yaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.LogLevel))
	required.Spec.Replicas = &options.Replicas
	required.Spec.Template.Spec.NodeSelector = options.NodeSelector

	// TODO based on the API server version we need to choose a different service serving cert key so that we serve with the correct cert
	required.Spec.Template.Spec.Volumes[0].Secret.SecretName = "webconsole-serving-cert"

	return resourceapply.ApplyDeployment(c.appsv1Client, required)
}
