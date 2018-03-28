package controller

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
	webconsolev1 "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
)

// between 3.10.0 and 3.10.1, we want to fix some naming mistakes.  During this time we need to ensure the
// 3.10.0 resources are correct and we need to spin up 3.10.1 *before* removing the old
func (c WebConsoleOperator) migrate10_0_to_10_1(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) error {
	errors := []error{}
	if _, err := c.ensureNamespace(); err != nil {
		errors = append(errors, err)
	}

	if _, err := resourceapply.ApplyServiceAccount(c.corev1Client,
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: "openshift-web-console", Name: "web-console"}}); err != nil {
		errors = append(errors, err)
	}

	// keep 3.10.0 up to date in case we have to do things like change the config
	if err := c.sync10_0(operatorConfig); err != nil {
		errors = append(errors, err)
	}

	// create the 3.10.1 resources
	modifiedConfig, err := c.ensureWebConsoleConfigMap10_1(operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}
	modifiedDeployment, err := c.ensureWebConsoleDeployment10_1(operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}

	// if we modified the 3.10.1 resources, then we have to wait for them to resettle.  Return and we'll be requeued on the watch
	if modifiedConfig || modifiedDeployment {
		// TODO update intermediate status and return
		return utilerrors.NewAggregate(errors)
	}
	if len(errors) > 0 {
		return utilerrors.NewAggregate(errors)
	}

	// TODO check to see if the deployment has ready pods
	newDeployment, err := c.appsv1Client.Deployments("openshift-web-console").Get("web-console", metav1.GetOptions{})
	if err != nil {
		// TODO update intermediate status
		return err
	}
	deployment10_1IsReady := newDeployment.Status.ReadyReplicas > 0
	if !deployment10_1IsReady {
		// TODO update intermediate status
		return utilerrors.NewAggregate(errors)
	}

	// after the new deployment has ready pods, we can swap the service and remove the old resources
	// TODO check the apiserver version to make this choice
	if _, err := resourceapply.ApplyService(c.corev1Client, resourceread.ReadServiceOrDie([]byte(service10_1_pre10_2APIServerYaml))); err != nil {
		errors = append(errors, err)
	}

	// if we have errors at this point, we cannot remove the 3.10.0 resources
	if len(errors) > 0 {
		// TODO update status
		return utilerrors.NewAggregate(errors)
	}

	if err := c.appsv1Client.Deployments("openshift-web-console").Delete("webconsole", nil); err != nil {
		errors = append(errors, err)
	}

	if err := c.corev1Client.ConfigMaps("openshift-web-console").Delete("webconsole-config", nil); err != nil {
		errors = append(errors, err)
	}

	// set the status
	operatorConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		operatorConfig.Status.LastUnsuccessfulRunErrors = append(operatorConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		operatorConfig.Status.LastSuccessfulVersion = operatorConfig.Spec.Version
	}
	if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
}

func (c WebConsoleOperator) sync10_1(operatorConfig *webconsolev1.OpenShiftWebConsoleConfig) error {
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
	if _, err := c.ensureWebConsoleDeployment10_1(operatorConfig.Spec); err != nil {
		errors = append(errors, err)
	}

	// set the status
	operatorConfig.Status.LastUnsuccessfulRunErrors = []string{}
	for _, err := range errors {
		operatorConfig.Status.LastUnsuccessfulRunErrors = append(operatorConfig.Status.LastUnsuccessfulRunErrors, err.Error())
	}
	if len(errors) == 0 {
		operatorConfig.Status.LastSuccessfulVersion = operatorConfig.Spec.Version
	}
	if _, err := c.operatorConfigClient.OpenShiftWebConsoleConfigs().Update(operatorConfig); err != nil {
		// if we had no other errors, then return this error so we can re-apply and then re-set the status
		if len(errors) == 0 {
			return err
		}
		utilruntime.HandleError(err)
	}

	return utilerrors.NewAggregate(errors)
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

func (c WebConsoleOperator) ensureWebConsoleDeployment10_1(options webconsolev1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	required := resourceread.ReadDeploymentOrDie([]byte(deployment10_1Yaml))
	required.Spec.Template.Spec.Containers[0].Image = options.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.LogLevel))
	required.Spec.Replicas = &options.Replicas
	required.Spec.Template.Spec.NodeSelector = options.NodeSelector

	// TODO based on the API server version we need to choose a different service serving cert key so that we serve with the correct cert
	required.Spec.Template.Spec.Volumes[0].Secret.SecretName = "webconsole-serving-cert"

	return resourceapply.ApplyDeployment(c.appsv1Client, required)
}
