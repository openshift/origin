package apiserver

import (
	"fmt"
	"reflect"

	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
)

// APIServerLister lists APIServer information and allows resources to be synced
type APIServerLister interface {
	APIServerLister() configlistersv1.APIServerLister
	PreRunHasSynced() []cache.InformerSynced
}

// ObserveTLSSecurityProfile observes APIServer.Spec.TLSSecurityProfile field and sets
// the ServingInfo.MinTLSVersion, ServingInfo.CipherSuites fields of observed config
func ObserveTLSSecurityProfile(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(genericListers, recorder, existingConfig, []string{"servingInfo", "minTLSVersion"}, []string{"servingInfo", "cipherSuites"})
}

// ObserveTLSSecurityProfileToArguments observes APIServer.Spec.TLSSecurityProfile field and sets
// the tls-min-version and tls-cipher-suites fileds of observedConfig.apiServerArguments
func ObserveTLSSecurityProfileToArguments(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerTLSSecurityProfileObservations(genericListers, recorder, existingConfig, []string{"apiServerArguments", "tls-min-version"}, []string{"apiServerArguments", "tls-cipher-suites"})
}

func innerTLSSecurityProfileObservations(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}, minTLSVersionPath, cipherSuitesPath []string) (ret map[string]interface{}, _ []error) {
	defer func() {
		ret = configobserver.Pruned(ret, minTLSVersionPath, cipherSuitesPath)
	}()

	listers := genericListers.(APIServerLister)
	errs := []error{}

	currentMinTLSVersion, _, versionErr := unstructured.NestedString(existingConfig, minTLSVersionPath...)
	if versionErr != nil {
		errs = append(errs, fmt.Errorf("failed to retrieve spec.servingInfo.minTLSVersion: %v", versionErr))
		// keep going on read error from existing config
	}

	currentCipherSuites, _, suitesErr := unstructured.NestedStringSlice(existingConfig, cipherSuitesPath...)
	if suitesErr != nil {
		errs = append(errs, fmt.Errorf("failed to retrieve spec.servingInfo.cipherSuites: %v", suitesErr))
		// keep going on read error from existing config
	}

	apiServer, err := listers.APIServerLister().Get("cluster")
	if errors.IsNotFound(err) {
		klog.Warningf("apiserver.config.openshift.io/cluster: not found")
		apiServer = &configv1.APIServer{}
	} else if err != nil {
		return existingConfig, append(errs, err)
	}

	observedConfig := map[string]interface{}{}
	observedMinTLSVersion, observedCipherSuites := getSecurityProfileCiphers(apiServer.Spec.TLSSecurityProfile)
	if err = unstructured.SetNestedField(observedConfig, observedMinTLSVersion, minTLSVersionPath...); err != nil {
		return existingConfig, append(errs, err)
	}
	if err = unstructured.SetNestedStringSlice(observedConfig, observedCipherSuites, cipherSuitesPath...); err != nil {
		return existingConfig, append(errs, err)
	}

	if observedMinTLSVersion != currentMinTLSVersion {
		recorder.Eventf("ObserveTLSSecurityProfile", "minTLSVersion changed to %s", observedMinTLSVersion)
	}
	if !reflect.DeepEqual(observedCipherSuites, currentCipherSuites) {
		recorder.Eventf("ObserveTLSSecurityProfile", "cipherSuites changed to %q", observedCipherSuites)
	}

	return observedConfig, errs
}

// Extracts the minimum TLS version and cipher suites from TLSSecurityProfile object,
// Converts the ciphers to IANA names as supported by Kube ServingInfo config.
// If profile is nil, returns config defined by the Intermediate TLS Profile
func getSecurityProfileCiphers(profile *configv1.TLSSecurityProfile) (string, []string) {
	var profileType configv1.TLSProfileType
	if profile == nil {
		profileType = configv1.TLSProfileIntermediateType
	} else {
		profileType = profile.Type
	}

	var profileSpec *configv1.TLSProfileSpec
	if profileType == configv1.TLSProfileCustomType {
		if profile.Custom != nil {
			profileSpec = &profile.Custom.TLSProfileSpec
		}
	} else {
		profileSpec = configv1.TLSProfiles[profileType]
	}

	// nothing found / custom type set but no actual custom spec
	if profileSpec == nil {
		profileSpec = configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	}

	// need to remap all Ciphers to their respective IANA names used by Go
	return string(profileSpec.MinTLSVersion), crypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers)
}
