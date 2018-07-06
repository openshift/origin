package operator

import (
	"bytes"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcecread"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
	"github.com/openshift/service-serving-cert-signer/pkg/operator/v310_00_assets"
)

// most of the time the sync method will be good for a large span of minor versions
func sync_v310_00_to_latest(c ServiceCertSignerOperator, operatorConfig *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity) (operatorsv1alpha1.VersionAvailablity, []error) {
	signingVersionAvailability, signingErrors := sync_v310_00_to_latest_SigningController(c, operatorConfig, previousAvailability)
	apiServiceInjectorVersionAvailability, apiServiceInjectorErrors := sync_v310_00_to_latest_APIServiceController(c, operatorConfig, previousAvailability)

	allErrors := []error{}
	allErrors = append(allErrors, signingErrors...)
	allErrors = append(allErrors, apiServiceInjectorErrors...)

	mergedVersionAvailability := operatorsv1alpha1.VersionAvailablity{
		Version: operatorConfig.Spec.Version,
	}
	mergedVersionAvailability.Generations = append(mergedVersionAvailability.Generations, signingVersionAvailability.Generations...)
	mergedVersionAvailability.Generations = append(mergedVersionAvailability.Generations, apiServiceInjectorVersionAvailability.Generations...)
	if signingVersionAvailability.UpdatedReplicas > 0 && apiServiceInjectorVersionAvailability.UpdatedReplicas > 0 {
		mergedVersionAvailability.UpdatedReplicas = 1
	}
	if signingVersionAvailability.ReadyReplicas > 0 && apiServiceInjectorVersionAvailability.ReadyReplicas > 0 {
		mergedVersionAvailability.ReadyReplicas = 1
	}

	return mergedVersionAvailability, allErrors
}

func sync_v310_00_to_latest_APIServiceController(c ServiceCertSignerOperator, operatorConfig *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity) (operatorsv1alpha1.VersionAvailablity, []error) {
	versionAvailability := operatorsv1alpha1.VersionAvailablity{
		Version: operatorConfig.Spec.Version,
	}

	errors := []error{}
	var err error

	requiredNamespace := resourceread.ReadNamespaceV1OrDie(v310_00_assets.MustAsset("v3.10.0/apiservice-cabundle-controller/ns.yaml"))
	_, _, err = resourceapply.ApplyNamespace(c.corev1Client, requiredNamespace)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "ns", err))
	}

	requiredService := resourceread.ReadServiceV1OrDie(v310_00_assets.MustAsset("v3.10.0/apiservice-cabundle-controller/svc.yaml"))
	_, _, err = resourceapply.ApplyService(c.corev1Client, requiredService)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "svc", err))
	}

	requiredSA := resourceread.ReadServiceAccountV1OrDie(v310_00_assets.MustAsset("v3.10.0/apiservice-cabundle-controller/sa.yaml"))
	_, saModified, err := resourceapply.ApplyServiceAccount(c.corev1Client, requiredSA)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "sa", err))
	}

	// TODO create a new configmap whenever the data value changes
	_, configMapModified, err := ensureAPIServiceInjectorConfigMap_v310_00_to_latest(c, operatorConfig.Spec)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "cm", err))
	}

	_, signingCABundleModified, err := manageSigningCABundle(c)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "signing-key", err))
	}

	forceDeployment := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration
	if saModified { // SA modification can cause new tokens
		forceDeployment = true
	}
	if signingCABundleModified {
		forceDeployment = true
	}
	if configMapModified {
		forceDeployment = true
	}

	// we have attempted to update our configmaps and secrets, now it is time to create the DS
	// TODO check basic preconditions here
	actualDeployment, _, err := ensureAPIServiceInjectorDeployment_v310_00_to_latest(c, operatorConfig, previousAvailability, forceDeployment)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "deployment", err))
	}
	if actualDeployment != nil {
		versionAvailability.UpdatedReplicas = actualDeployment.Status.UpdatedReplicas
		versionAvailability.ReadyReplicas = actualDeployment.Status.ReadyReplicas
		versionAvailability.Generations = []operatorsv1alpha1.GenerationHistory{
			{
				Group: "apps", Resource: "Deployment",
				Namespace: targetNamespaceName, Name: "apiservice-cabundle-injector",
				LastGeneration: actualDeployment.ObjectMeta.Generation,
			},
		}
	}

	v1alpha1helpers.SetErrors(&versionAvailability, errors...)

	return versionAvailability, errors
}

func sync_v310_00_to_latest_SigningController(c ServiceCertSignerOperator, operatorConfig *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity) (operatorsv1alpha1.VersionAvailablity, []error) {
	versionAvailability := operatorsv1alpha1.VersionAvailablity{
		Version: operatorConfig.Spec.Version,
	}

	errors := []error{}
	var err error

	requiredNamespace := resourceread.ReadNamespaceV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/ns.yaml"))
	_, _, err = resourceapply.ApplyNamespace(c.corev1Client, requiredNamespace)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "ns", err))
	}

	requiredService := resourceread.ReadServiceV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/svc.yaml"))
	_, _, err = resourceapply.ApplyService(c.corev1Client, requiredService)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "svc", err))
	}

	requiredSA := resourceread.ReadServiceAccountV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/sa.yaml"))
	_, saModified, err := resourceapply.ApplyServiceAccount(c.corev1Client, requiredSA)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "sa", err))
	}

	// TODO create a new configmap whenever the data value changes
	_, configMapModified, err := ensureServingSignerConfigMap_v310_00_to_latest(c, operatorConfig.Spec)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "cm", err))
	}

	_, signingSecretModified, err := manageSigningSecret(c)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "signing-key", err))
	}

	forceDeployment := operatorConfig.ObjectMeta.Generation != operatorConfig.Status.ObservedGeneration
	if saModified { // SA modification can cause new tokens
		forceDeployment = true
	}
	if signingSecretModified {
		forceDeployment = true
	}
	if configMapModified {
		forceDeployment = true
	}

	// our configmaps and secrets are in order, now it is time to create the DS
	// TODO check basic preconditions here
	actualDeployment, _, err := ensureServingSignerDeployment_v310_00_to_latest(c, operatorConfig, previousAvailability, forceDeployment)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "deployment", err))
	}
	if actualDeployment != nil {
		versionAvailability.UpdatedReplicas = actualDeployment.Status.UpdatedReplicas
		versionAvailability.ReadyReplicas = actualDeployment.Status.ReadyReplicas
		versionAvailability.Generations = []operatorsv1alpha1.GenerationHistory{
			{
				Group: "apps", Resource: "Deployment",
				Namespace: targetNamespaceName, Name: "service-serving-cert-signer",
				LastGeneration: actualDeployment.ObjectMeta.Generation,
			},
		}
	}

	v1alpha1helpers.SetErrors(&versionAvailability, errors...)

	return versionAvailability, errors
}

// TODO manage rotation in addition to initial creation
func manageSigningSecret(c ServiceCertSignerOperator) (*corev1.Secret, bool, error) {
	secret := resourceread.ReadSecretV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/signing-secret.yaml"))
	existing, err := c.corev1Client.Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return existing, false, err
	}

	ca, err := crypto.MakeCAConfig(serviceServingCertSignerName(), 10)
	if err != nil {
		return existing, false, err
	}

	certBytes := &bytes.Buffer{}
	keyBytes := &bytes.Buffer{}
	if err := ca.WriteCertConfig(certBytes, keyBytes); err != nil {
		return existing, false, err
	}

	secret.Data["tls.crt"] = certBytes.Bytes()
	secret.Data["tls.key"] = keyBytes.Bytes()

	return resourceapply.ApplySecret(c.corev1Client, secret)
}

func ensureServingSignerConfigMap_v310_00_to_latest(c ServiceCertSignerOperator, options scsv1alpha1.ServiceCertSignerOperatorConfigSpec) (*corev1.ConfigMap, bool, error) {
	// TODO use an unstructured object to merge configs
	config, err := readServiceServingCertSignerConfig(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/defaultconfig.yaml"))
	if err != nil {
		return nil, false, err
	}
	configBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, config)
	if err != nil {
		return nil, false, err
	}

	requiredConfigMap := resourceread.ReadConfigMapV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/cm.yaml"))
	const configKey = "controller-config.yaml"
	requiredConfigMap.Data[configKey] = string(configBytes)

	return resourceapply.ApplyConfigMap(c.corev1Client, requiredConfigMap)
}

func ensureServingSignerDeployment_v310_00_to_latest(c ServiceCertSignerOperator, options *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/deployment.yaml"))
	required.Spec.Template.Spec.Containers[0].Image = options.Spec.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.Spec.Logging.Level))

	generation := int64(-1)
	if previousAvailability != nil {
		for _, curr := range previousAvailability.Generations {
			if curr.Name == "service-serving-cert-signer" {
				generation = curr.LastGeneration
			}
		}
	}
	return resourceapply.ApplyDeployment(c.appsv1Client, required, generation, forceDeployment)
}

func serviceServingCertSignerName() string {
	return fmt.Sprintf("%s@%d", "openshift-service-serving-signer", time.Now().Unix())
}

func ensureAPIServiceInjectorConfigMap_v310_00_to_latest(c ServiceCertSignerOperator, options scsv1alpha1.ServiceCertSignerOperatorConfigSpec) (*corev1.ConfigMap, bool, error) {
	// TODO use an unstructured object to merge configs
	config, err := readServiceServingCertSignerConfig(v310_00_assets.MustAsset("v3.10.0/apiservice-cabundle-controller/defaultconfig.yaml"))
	if err != nil {
		return nil, false, err
	}
	configBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, config)
	if err != nil {
		return nil, false, err
	}

	requiredConfigMap := resourceread.ReadConfigMapV1OrDie(v310_00_assets.MustAsset("v3.10.0/apiservice-cabundle-controller/cm.yaml"))
	const configKey = "controller-config.yaml"
	requiredConfigMap.Data[configKey] = string(configBytes)

	return resourceapply.ApplyConfigMap(c.corev1Client, requiredConfigMap)
}

func ensureAPIServiceInjectorDeployment_v310_00_to_latest(c ServiceCertSignerOperator, options *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(v310_00_assets.MustAsset("v3.10.0/apiservice-cabundle-controller/deployment.yaml"))
	required.Spec.Template.Spec.Containers[0].Image = options.Spec.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.Spec.Logging.Level))

	generation := int64(-1)
	if previousAvailability != nil {
		for _, curr := range previousAvailability.Generations {
			if curr.Name == "apiservice-cabundle-injector" {
				generation = curr.LastGeneration
			}
		}
	}
	return resourceapply.ApplyDeployment(c.appsv1Client, required, generation, forceDeployment)
}

// TODO manage rotation in addition to initial creation
func manageSigningCABundle(c ServiceCertSignerOperator) (*corev1.ConfigMap, bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v310_00_assets.MustAsset("v3.10.0/apiservice-cabundle-controller/signing-cabundle.yaml"))
	existing, err := c.corev1Client.ConfigMaps(configMap.Namespace).Get(configMap.Name, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		return existing, false, err
	}

	secret := resourceread.ReadSecretV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/signing-secret.yaml"))
	currentSigningKeySecret, err := c.corev1Client.Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return existing, false, err
	}
	if err != nil {
		return existing, false, err
	}
	if len(currentSigningKeySecret.Data["tls.crt"]) == 0 {
		return existing, false, err
	}

	configMap.Data["cabundle.crt"] = string(currentSigningKeySecret.Data["tls.crt"])

	return resourceapply.ApplyConfigMap(c.corev1Client, configMap)
}
