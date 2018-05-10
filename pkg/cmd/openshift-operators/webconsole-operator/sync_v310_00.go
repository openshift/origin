package webconsole_operator

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	operatorsv1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/operators/v1alpha1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/apis/operators/v1alpha1helpers"
	webconsolev1alpha1 "github.com/openshift/origin/pkg/cmd/openshift-operators/apis/webconsole/v1alpha1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceapply"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourceread"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/v310_00"
)

// most of the time the sync method will be good for a large span of minor versions
func sync_v310_00_to_00(c WebConsoleOperator, operatorConfig *webconsolev1alpha1.OpenShiftWebConsoleConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity) (operatorsv1alpha1.VersionAvailablity, []error) {
	versionAvailability := operatorsv1alpha1.VersionAvailablity{
		Version: operatorConfig.Spec.Version,
	}

	errors := []error{}
	var err error

	_, err = ensureNamespace_v310_00_to_00(c)
	if err != nil {
		errors = append(errors, err)
	}

	_, err = ensureService_v310_00_to_00(c)
	if err != nil {
		errors = append(errors, err)
	}

	_, err = ensureServiceAccount_v310_00_to_00(c)
	if err != nil {
		errors = append(errors, err)
	}

	// TODO create a new configmap whenever the data value changes
	configMapModified, err := ensureConfigMap_v310_00_to_00(c, operatorConfig.Spec)
	if err != nil {
		errors = append(errors, err)
	}

	forceDeployment := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration
	if configMapModified {
		forceDeployment = true
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	actualDeployment, _, err := ensureDeployment_v310_00_to_00(c, operatorConfig, previousAvailability, forceDeployment)
	if err != nil {
		errors = append(errors, err)
	}
	if actualDeployment != nil {
		versionAvailability.UpdatedReplicas = actualDeployment.Status.UpdatedReplicas
		versionAvailability.ReadyReplicas = actualDeployment.Status.ReadyReplicas
		versionAvailability.Generations = []operatorsv1alpha1.GenerationHistory{
			{
				Group: "apps", Resource: "Deployment",
				Namespace: targetNamespaceName, Name: "webconsole",
				LastGeneration: actualDeployment.ObjectMeta.Generation,
			},
		}
	}

	v1alpha1helpers.SetErrors(&versionAvailability, errors...)

	return versionAvailability, errors
}

func ensureNamespace_v310_00_to_00(c WebConsoleOperator) (bool, error) {
	required := resourceread.ReadNamespaceOrDie([]byte(v310_00.NamespaceYaml))
	return resourceapply.ApplyNamespace(c.corev1Client, required)
}

func ensureService_v310_00_to_00(c WebConsoleOperator) (bool, error) {
	required := resourceread.ReadServiceOrDie([]byte(v310_00.ServiceYaml))
	return resourceapply.ApplyService(c.corev1Client, required)
}

func ensureServiceAccount_v310_00_to_00(c WebConsoleOperator) (bool, error) {
	required := resourceread.ReadServiceAccountOrDie([]byte(v310_00.ServiceAccountYaml))
	return resourceapply.ApplyServiceAccount(c.corev1Client, required)
}

func ensureConfigMap_v310_00_to_00(c WebConsoleOperator, options webconsolev1alpha1.OpenShiftWebConsoleConfigSpec) (bool, error) {
	requiredConfig, err := ensureWebConsoleConfig(v310_00.WebConsoleConfig, options)
	if err != nil {
		return false, err
	}

	newWebConsoleConfig, err := runtime.Encode(webconsoleCodecs.LegacyCodec(webconsoleconfigv1.SchemeGroupVersion), requiredConfig)
	if err != nil {
		return false, err
	}
	requiredConfigMap := resourceread.ReadConfigMapOrDie([]byte(v310_00.ConfigMapYaml))
	requiredConfigMap.Data[v310_00.ConfigConfigMapKey] = string(newWebConsoleConfig)

	return resourceapply.ApplyConfigMap(c.corev1Client, requiredConfigMap)
}

func ensureDeployment_v310_00_to_00(c WebConsoleOperator, options *webconsolev1alpha1.OpenShiftWebConsoleConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentOrDie([]byte(v310_00.DeploymentYaml))
	required.Spec.Template.Spec.Containers[0].Image = options.Spec.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.Spec.Logging.Level))
	required.Spec.Replicas = &options.Spec.Replicas
	required.Spec.Template.Spec.NodeSelector = options.Spec.NodeSelector

	generation := int64(-1)
	if previousAvailability != nil {
		for _, curr := range previousAvailability.Generations {
			if curr.Name == "webconsole" {
				generation = curr.LastGeneration
			}
		}
	}
	return resourceapply.ApplyDeployment(c.appsv1Client, required, generation, forceDeployment)
}
