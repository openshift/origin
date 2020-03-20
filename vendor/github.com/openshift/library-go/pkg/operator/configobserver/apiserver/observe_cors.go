package apiserver

import (
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

var clusterDefaultCORSAllowedOrigins = []string{
	`//127\.0\.0\.1(:|$)`,
	`//localhost(:|$)`,
}

// ObserveAdditionalCORSAllowedOrigins observes the additionalCORSAllowedOrigins field
// of the APIServer resource and sets the corsAllowedOrigins field of observedConfig
func ObserveAdditionalCORSAllowedOrigins(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerObserveAdditionalCORSAllowedOrigins(genericListers, recorder, existingConfig, []string{"corsAllowedOrigins"})
}

// ObserveAdditionalCORSAllowedOriginsToArguments observes the additionalCORSAllowedOrigins field
// of the APIServer resource and sets the cors-allowed-origins field in observedConfig.apiServerArguments
func ObserveAdditionalCORSAllowedOriginsToArguments(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}) (map[string]interface{}, []error) {
	return innerObserveAdditionalCORSAllowedOrigins(genericListers, recorder, existingConfig, []string{"apiServerArguments", "cors-allowed-origins"})
}

func innerObserveAdditionalCORSAllowedOrigins(genericListers configobserver.Listers, recorder events.Recorder, existingConfig map[string]interface{}, corsAllowedOriginsPath []string) (ret map[string]interface{}, _ []error) {
	defer func() {
		ret = configobserver.Pruned(ret, corsAllowedOriginsPath)
	}()

	lister := genericListers.(APIServerLister)
	errs := []error{}
	defaultConfig := map[string]interface{}{}
	if err := unstructured.SetNestedStringSlice(defaultConfig, clusterDefaultCORSAllowedOrigins, corsAllowedOriginsPath...); err != nil {
		// this should not happen
		return existingConfig, append(errs, err)
	}

	// grab the current CORS origins to later check whether they were updated
	currentCORSAllowedOrigins, _, err := unstructured.NestedStringSlice(existingConfig, corsAllowedOriginsPath...)
	if err != nil {
		errs = append(errs, err)
		// keep going on read error from existing config
	}
	currentCORSSet := sets.NewString(currentCORSAllowedOrigins...)
	currentCORSSet.Insert(clusterDefaultCORSAllowedOrigins...)

	observedConfig := map[string]interface{}{}
	apiServer, err := lister.APIServerLister().Get("cluster")
	if errors.IsNotFound(err) {
		klog.Warningf("apiserver.config.openshift.io/cluster: not found")
		return defaultConfig, errs
	}
	if err != nil {
		// return existingConfig here in case err is just a transient error so
		// that we don't rewrite the config that was observed previously
		return existingConfig, append(errs, err)
	}

	newCORSSet := sets.NewString(clusterDefaultCORSAllowedOrigins...)
	newCORSSet.Insert(apiServer.Spec.AdditionalCORSAllowedOrigins...)
	if err := unstructured.SetNestedStringSlice(observedConfig, newCORSSet.List(), corsAllowedOriginsPath...); err != nil {
		return existingConfig, append(errs, err)
	}

	if !currentCORSSet.Equal(newCORSSet) {
		recorder.Eventf("ObserveAdditionalCORSAllowedOrigins", "corsAllowedOrigins changed to %q", newCORSSet.List())
	}

	return observedConfig, errs
}
