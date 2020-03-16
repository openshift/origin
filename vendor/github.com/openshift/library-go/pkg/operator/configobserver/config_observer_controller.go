package configobserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/imdario/mergo"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/cache"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// Listers is an interface which will be passed to the config observer funcs.  It is expected to be hard-cast to the "correct" type
type Listers interface {
	// ResourceSyncer can be used to copy content from one namespace to another
	ResourceSyncer() resourcesynccontroller.ResourceSyncer
	PreRunHasSynced() []cache.InformerSynced
}

// ObserveConfigFunc observes configuration and returns the observedConfig. This function should not return an
// observedConfig that would cause the service being managed by the operator to crash. For example, if a required
// configuration key cannot be observed, consider reusing the configuration key's previous value. Errors that occur
// while attempting to generate the observedConfig should be returned in the errs slice.
type ObserveConfigFunc func(listers Listers, recorder events.Recorder, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error)

type ConfigObserver struct {
	// observers are called in an undefined order and their results are merged to
	// determine the observed configuration.
	observers []ObserveConfigFunc

	operatorClient v1helpers.OperatorClient
	// listers are used by config observers to retrieve necessary resources
	listers Listers
}

func NewConfigObserver(
	operatorClient v1helpers.OperatorClient,
	eventRecorder events.Recorder,
	listers Listers,
	observers ...ObserveConfigFunc,
) factory.Controller {
	c := &ConfigObserver{
		operatorClient: operatorClient,
		observers:      observers,
		listers:        listers,
	}
	return factory.New().ResyncEvery(time.Second).WithSync(c.sync).WithInformers(listersToInformer(listers)...).ToController("ConfigObserver", eventRecorder.WithComponentSuffix("config-observer"))
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c ConfigObserver) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	originalSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}
	spec := originalSpec.DeepCopy()

	// don't worry about errors.  If we can't decode, we'll simply stomp over the field.
	existingConfig := map[string]interface{}{}
	if err := json.NewDecoder(bytes.NewBuffer(spec.ObservedConfig.Raw)).Decode(&existingConfig); err != nil {
		klog.V(4).Infof("decode of existing config failed with error: %v", err)
	}

	var errs []error
	var observedConfigs []map[string]interface{}
	for _, i := range rand.Perm(len(c.observers)) {
		var currErrs []error
		observedConfig, currErrs := c.observers[i](c.listers, syncCtx.Recorder(), existingConfig)
		observedConfigs = append(observedConfigs, observedConfig)
		errs = append(errs, currErrs...)
	}

	mergedObservedConfig := map[string]interface{}{}
	for _, observedConfig := range observedConfigs {
		if err := mergo.Merge(&mergedObservedConfig, observedConfig); err != nil {
			klog.Warningf("merging observed config failed: %v", err)
		}
	}

	reverseMergedObservedConfig := map[string]interface{}{}
	for i := len(observedConfigs) - 1; i >= 0; i-- {
		if err := mergo.Merge(&reverseMergedObservedConfig, observedConfigs[i]); err != nil {
			klog.Warningf("merging observed config failed: %v", err)
		}
	}

	if !equality.Semantic.DeepEqual(mergedObservedConfig, reverseMergedObservedConfig) {
		errs = append(errs, errors.New("non-deterministic config observation detected"))
	}

	if !equality.Semantic.DeepEqual(existingConfig, mergedObservedConfig) {
		syncCtx.Recorder().Eventf("ObservedConfigChanged", "Writing updated observed config: %v", diff.ObjectDiff(existingConfig, mergedObservedConfig))
		if _, _, err := v1helpers.UpdateSpec(c.operatorClient, v1helpers.UpdateObservedConfigFn(mergedObservedConfig)); err != nil {
			// At this point we failed to write the updated config. If we are permanently broken, do not pile the errors from observers
			// but instead reset the errors and only report single error condition.
			errs = []error{fmt.Errorf("error writing updated observed config: %v", err)}
			syncCtx.Recorder().Warningf("ObservedConfigWriteError", "Failed to write observed config: %v", err)
		}
	}
	configError := v1helpers.NewMultiLineAggregate(errs)

	// update failing condition
	cond := operatorv1.OperatorCondition{
		Type:   condition.ConfigObservationDegradedConditionType,
		Status: operatorv1.ConditionFalse,
	}
	if configError != nil {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Error"
		cond.Message = configError.Error()
	}
	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		return updateError
	}

	return configError
}

// listersToInformer converts the Listers interface to informer with empty AddEventHandler as we only care about synced caches in the Run.
func listersToInformer(l Listers) []factory.Informer {
	result := make([]factory.Informer, len(l.PreRunHasSynced()))
	for i := range l.PreRunHasSynced() {
		result[i] = &listerInformer{cacheSynced: l.PreRunHasSynced()[i]}
	}
	return result
}

type listerInformer struct {
	cacheSynced cache.InformerSynced
}

func (l *listerInformer) AddEventHandler(cache.ResourceEventHandler) {
	return
}

func (l *listerInformer) HasSynced() bool {
	return l.cacheSynced()
}
