package operator

import (
	"fmt"
	"strings"

	"github.com/blang/semver"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	coreclientv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacclientv1 "k8s.io/client-go/kubernetes/typed/rbac/v1"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	scsclientv1alpha1 "github.com/openshift/client-go/servicecertsigner/clientset/versioned/typed/servicecertsigner/v1alpha1"
	scsinformerv1alpha1 "github.com/openshift/client-go/servicecertsigner/informers/externalversions/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
	"github.com/openshift/library-go/pkg/operator/versioning"
	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/operator"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

const targetNamespaceName = "openshift-service-cert-signer"

type serviceCertSignerOperator struct {
	operatorConfigClient scsclientv1alpha1.ServiceCertSignerOperatorConfigsGetter

	appsv1Client appsclientv1.AppsV1Interface
	corev1Client coreclientv1.CoreV1Interface
	rbacv1Client rbacclientv1.RbacV1Interface
}

func NewServiceCertSignerOperator(
	serviceCertSignerConfigInformer scsinformerv1alpha1.ServiceCertSignerOperatorConfigInformer,
	namespacedKubeInformers informers.SharedInformerFactory,
	operatorConfigClient scsclientv1alpha1.ServiceCertSignerOperatorConfigsGetter,
	appsv1Client appsclientv1.AppsV1Interface,
	corev1Client coreclientv1.CoreV1Interface,
	rbacv1Client rbacclientv1.RbacV1Interface,
) operator.Runner {
	c := &serviceCertSignerOperator{
		operatorConfigClient: operatorConfigClient,

		appsv1Client: appsv1Client,
		corev1Client: corev1Client,
		rbacv1Client: rbacv1Client,
	}

	configEvents := operator.FilterByNames(api.OperatorConfigInstanceName)
	configMapEvents := operator.FilterByNames(
		api.SignerControllerConfigMapName,
		api.APIServiceInjectorConfigMapName,
		api.ConfigMapInjectorConfigMapName,
		api.SigningCABundleConfigMapName,
	)
	saEvents := operator.FilterByNames(
		api.SignerControllerSAName,
		api.APIServiceInjectorSAName,
		api.ConfigMapInjectorSAName,
	)
	serviceEvents := operator.FilterByNames(api.SignerControllerServiceName)
	secretEvents := operator.FilterByNames(api.SignerControllerSecretName)
	deploymentEvents := operator.FilterByNames(
		api.SignerControllerDeploymentName,
		api.APIServiceInjectorDeploymentName,
		api.ConfigMapInjectorDeploymentName,
	)
	namespaceEvents := operator.FilterByNames(targetNamespaceName)

	return operator.New("ServiceCertSignerOperator", c,
		operator.WithInformer(serviceCertSignerConfigInformer, configEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().ConfigMaps(), configMapEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().ServiceAccounts(), saEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Services(), serviceEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Secrets(), secretEvents),
		operator.WithInformer(namespacedKubeInformers.Apps().V1().Deployments(), deploymentEvents),
		operator.WithInformer(namespacedKubeInformers.Core().V1().Namespaces(), namespaceEvents),
	)
}

func (c serviceCertSignerOperator) Key() (metav1.Object, error) {
	return c.operatorConfigClient.ServiceCertSignerOperatorConfigs().Get(api.OperatorConfigInstanceName, metav1.GetOptions{})
}

func (c serviceCertSignerOperator) Sync(obj metav1.Object) error {
	operatorConfig := obj.(*scsv1alpha1.ServiceCertSignerOperatorConfig)

	switch operatorConfig.Spec.ManagementState {
	case operatorsv1alpha1.Unmanaged:
		return nil

	case operatorsv1alpha1.Removed:
		// TODO probably need to watch until the NS is really gone
		if err := c.corev1Client.Namespaces().Delete(targetNamespaceName, nil); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		operatorConfig.Status.TaskSummary = "Remove"
		operatorConfig.Status.TargetAvailability = nil
		operatorConfig.Status.CurrentAvailability = nil
		operatorConfig.Status.Conditions = []operatorsv1alpha1.OperatorCondition{
			{
				Type:   operatorsv1alpha1.OperatorStatusTypeAvailable,
				Status: operatorsv1alpha1.ConditionFalse,
			},
		}
		if _, err := c.operatorConfigClient.ServiceCertSignerOperatorConfigs().Update(operatorConfig); err != nil {
			return err
		}
		return nil
	}

	operatorConfigOriginal := operatorConfig.DeepCopy()

	var currentActualVerion *semver.Version

	if operatorConfig.Status.CurrentAvailability != nil {
		ver, err := semver.Parse(operatorConfig.Status.CurrentAvailability.Version)
		if err != nil {
			utilruntime.HandleError(err)
		} else {
			currentActualVerion = &ver
		}
	}
	desiredVersion, err := semver.Parse(operatorConfig.Spec.Version)
	if err != nil {
		// TODO report failing status, we may actually attempt to do this in the "normal" error handling
		return err
	}

	v310_00_to_unknown := versioning.NewRangeOrDie("3.10.0", "3.10.1")

	errors := []error{}
	switch {
	case v310_00_to_unknown.BetweenOrEmpty(currentActualVerion) && v310_00_to_unknown.Between(&desiredVersion):
		var versionAvailability operatorsv1alpha1.VersionAvailability
		operatorConfig.Status.TaskSummary = "sync-[3.10.0,3.10.1)"
		operatorConfig.Status.TargetAvailability = nil
		versionAvailability, errors = sync_v311_00_to_latest(c, operatorConfig, operatorConfig.Status.CurrentAvailability)
		operatorConfig.Status.CurrentAvailability = &versionAvailability

	default:
		operatorConfig.Status.TaskSummary = "unrecognized"
		if _, err := c.operatorConfigClient.ServiceCertSignerOperatorConfigs().UpdateStatus(operatorConfig); err != nil {
			utilruntime.HandleError(err)
		}

		return fmt.Errorf("unrecognized state")
	}

	// given the VersionAvailability and the status.Version, we can compute availability
	availableCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeAvailable,
		Status: operatorsv1alpha1.ConditionUnknown,
	}
	if operatorConfig.Status.CurrentAvailability != nil && operatorConfig.Status.CurrentAvailability.ReadyReplicas > 0 {
		availableCondition.Status = operatorsv1alpha1.ConditionTrue
	} else {
		availableCondition.Status = operatorsv1alpha1.ConditionFalse
	}
	v1alpha1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, availableCondition)

	syncSuccessfulCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeSyncSuccessful,
		Status: operatorsv1alpha1.ConditionTrue,
	}
	if operatorConfig.Status.CurrentAvailability != nil && len(operatorConfig.Status.CurrentAvailability.Errors) > 0 {
		syncSuccessfulCondition.Status = operatorsv1alpha1.ConditionFalse
		syncSuccessfulCondition.Message = strings.Join(operatorConfig.Status.CurrentAvailability.Errors, "\n")
	}
	if operatorConfig.Status.TargetAvailability != nil && len(operatorConfig.Status.TargetAvailability.Errors) > 0 {
		syncSuccessfulCondition.Status = operatorsv1alpha1.ConditionFalse
		if len(syncSuccessfulCondition.Message) == 0 {
			syncSuccessfulCondition.Message = strings.Join(operatorConfig.Status.TargetAvailability.Errors, "\n")
		} else {
			syncSuccessfulCondition.Message = availableCondition.Message + "\n" + strings.Join(operatorConfig.Status.TargetAvailability.Errors, "\n")
		}
	}
	v1alpha1helpers.SetOperatorCondition(&operatorConfig.Status.Conditions, syncSuccessfulCondition)
	if syncSuccessfulCondition.Status == operatorsv1alpha1.ConditionTrue {
		operatorConfig.Status.ObservedGeneration = operatorConfig.ObjectMeta.Generation
	}

	if !apiequality.Semantic.DeepEqual(operatorConfigOriginal, operatorConfig) {
		if _, err := c.operatorConfigClient.ServiceCertSignerOperatorConfigs().UpdateStatus(operatorConfig); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}
