package operator

import (
	"bytes"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcemerge"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/openshift/service-serving-cert-signer/pkg/operator/v310_00_assets"
)

// syncSigningController_v311_00_to_latest takes care of synchronizing (not upgrading) the thing we're managing.
// most of the time the sync method will be good for a large span of minor versions
func syncSigningController_v311_00_to_latest(c serviceCertSignerOperator, operatorConfig *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailability) (operatorsv1alpha1.VersionAvailability, []error) {
	versionAvailability := operatorsv1alpha1.VersionAvailability{
		Version: operatorConfig.Spec.Version,
	}

	errors := []error{}
	var err error

	requiredNamespace := resourceread.ReadNamespaceV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/ns.yaml"))
	_, _, err = resourceapply.ApplyNamespace(c.corev1Client, requiredNamespace)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "ns", err))
	}

	requiredClusterRole := resourceread.ReadClusterRoleV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/clusterrole.yaml"))
	_, _, err = resourceapply.ApplyClusterRole(c.rbacv1Client, requiredClusterRole)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "svc", err))
	}

	requiredClusterRoleBinding := resourceread.ReadClusterRoleBindingV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/clusterrolebinding.yaml"))
	_, _, err = resourceapply.ApplyClusterRoleBinding(c.rbacv1Client, requiredClusterRoleBinding)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "svc", err))
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
	_, configMapModified, err := manageSigningConfigMap_v311_00_to_latest(c.corev1Client, operatorConfig)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "configmap", err))
	}

	_, signingSecretModified, err := manageSigningSecret_v311_00_to_latest(c.corev1Client)
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
	actualDeployment, _, err := manageSignerDeployment_v311_00_to_latest(c.appsv1Client, operatorConfig, previousAvailability, forceDeployment)
	if err != nil {
		errors = append(errors, fmt.Errorf("%q: %v", "deployment", err))
	}

	return resourcemerge.ApplyDeploymentGenerationAvailability(versionAvailability, actualDeployment, errors...), errors
}

func manageSigningConfigMap_v311_00_to_latest(client coreclientv1.ConfigMapsGetter, operatorConfig *scsv1alpha1.ServiceCertSignerOperatorConfig) (*corev1.ConfigMap, bool, error) {
	configMap := resourceread.ReadConfigMapV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/cm.yaml"))
	defaultConfig := v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/defaultconfig.yaml")
	requiredConfigMap, _, err := resourcemerge.MergeConfigMap(configMap, "controller-config.yaml", nil, defaultConfig, operatorConfig.Spec.ServiceServingCertSignerConfig.Raw)
	if err != nil {
		return nil, false, err
	}
	return resourceapply.ApplyConfigMap(client, requiredConfigMap)
}

// TODO manage rotation in addition to initial creation
func manageSigningSecret_v311_00_to_latest(client coreclientv1.SecretsGetter) (*corev1.Secret, bool, error) {
	secret := resourceread.ReadSecretV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/signing-secret.yaml"))
	existing, err := client.Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
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

	return resourceapply.ApplySecret(client, secret)
}

func manageSignerDeployment_v311_00_to_latest(client appsclientv1.DeploymentsGetter, options *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailability, forceDeployment bool) (*appsv1.Deployment, bool, error) {
	required := resourceread.ReadDeploymentV1OrDie(v310_00_assets.MustAsset("v3.10.0/service-serving-cert-signer-controller/deployment.yaml"))
	required.Spec.Template.Spec.Containers[0].Image = options.Spec.ImagePullSpec
	required.Spec.Template.Spec.Containers[0].Args = append(required.Spec.Template.Spec.Containers[0].Args, fmt.Sprintf("-v=%d", options.Spec.Logging.Level))

	return resourceapply.ApplyDeployment(client, required, resourcemerge.ExpectedDeploymentGeneration(required, previousAvailability), forceDeployment)
}

func serviceServingCertSignerName() string {
	return fmt.Sprintf("%s@%d", "openshift-service-serving-signer", time.Now().Unix())
}
