package operator

import (
	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
)

// sync_v311_00_to_latest takes care of synchronizing (not upgrading) the thing we're managing.
// most of the time the sync method will be good for a large span of minor versions
func sync_v311_00_to_latest(c ServiceCertSignerOperator, operatorConfig *scsv1alpha1.ServiceCertSignerOperatorConfig, previousAvailability *operatorsv1alpha1.VersionAvailablity) (operatorsv1alpha1.VersionAvailablity, []error) {
	signingVersionAvailability, signingErrors := syncSigningController_v311_00_to_latest(c, operatorConfig, previousAvailability)
	apiServiceInjectorVersionAvailability, apiServiceInjectorErrors := syncAPIServiceController_v311_00_to_latest(c, operatorConfig, previousAvailability)
	configMapCABundleInjectorVersionAvailability, configMapCABundleInjectorErrors := syncConfigMapCABundleController_v311_00_to_latest(c, operatorConfig, previousAvailability)

	allErrors := []error{}
	allErrors = append(allErrors, signingErrors...)
	allErrors = append(allErrors, apiServiceInjectorErrors...)
	allErrors = append(allErrors, configMapCABundleInjectorErrors...)

	mergedVersionAvailability := operatorsv1alpha1.VersionAvailablity{
		Version: operatorConfig.Spec.Version,
	}
	mergedVersionAvailability.Generations = append(mergedVersionAvailability.Generations, signingVersionAvailability.Generations...)
	mergedVersionAvailability.Generations = append(mergedVersionAvailability.Generations, apiServiceInjectorVersionAvailability.Generations...)
	mergedVersionAvailability.Generations = append(mergedVersionAvailability.Generations, configMapCABundleInjectorVersionAvailability.Generations...)
	if signingVersionAvailability.UpdatedReplicas > 0 && apiServiceInjectorVersionAvailability.UpdatedReplicas > 0 && configMapCABundleInjectorVersionAvailability.UpdatedReplicas > 0 {
		mergedVersionAvailability.UpdatedReplicas = 1
	}
	if signingVersionAvailability.ReadyReplicas > 0 && apiServiceInjectorVersionAvailability.ReadyReplicas > 0 && configMapCABundleInjectorVersionAvailability.ReadyReplicas > 0 {
		mergedVersionAvailability.ReadyReplicas = 1
	}
	for _, err := range allErrors {
		mergedVersionAvailability.Errors = append(mergedVersionAvailability.Errors, err.Error())
	}

	return mergedVersionAvailability, allErrors
}
