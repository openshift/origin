package configobserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/imdario/mergo"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/rand"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const operatorStatusTypeConfigObservationFailing = "ConfigObservationFailing"
const configObservationErrorConditionReason = "ConfigObservationError"
const configObserverWorkKey = "key"

type OperatorClient interface {
	GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error)
	UpdateOperatorSpec(string, *operatorv1.OperatorSpec) (spec *operatorv1.OperatorSpec, resourceVersion string, err error)
	UpdateOperatorStatus(string, *operatorv1.OperatorStatus) (status *operatorv1.OperatorStatus, resourceVersion string, err error)
}

// Listers is an interface which will be passed to the config observer funcs.  It is expected to be hard-cast to the "correct" type
type Listers interface {
	PreRunHasSynced() []cache.InformerSynced
}

// ObserveConfigFunc observes configuration and returns the observedConfig. This function should not return an
// observedConfig that would cause the service being managed by the operator to crash. For example, if a required
// configuration key cannot be observed, consider reusing the configuration key's previous value. Errors that occur
// while attempting to generate the observedConfig should be returned in the errs slice.
type ObserveConfigFunc func(listers Listers, existingConfig map[string]interface{}) (observedConfig map[string]interface{}, errs []error)

type ConfigObserver struct {
	operatorClient OperatorClient

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	rateLimiter flowcontrol.RateLimiter
	// observers are called in an undefined order and their results are merged to
	// determine the observed configuration.
	observers []ObserveConfigFunc

	// listers are used by config observers to retrieve necessary resources
	listers Listers
}

func NewConfigObserver(
	operatorClient OperatorClient,
	listers Listers,
	observers ...ObserveConfigFunc,
) *ConfigObserver {
	return &ConfigObserver{
		operatorClient: operatorClient,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ConfigObserver"),

		rateLimiter: flowcontrol.NewTokenBucketRateLimiter(0.05 /*3 per minute*/, 4),
		observers:   observers,
		listers:     listers,
	}
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c ConfigObserver) sync() error {
	originalSpec, originalStatus, resourceVersion, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}
	spec := originalSpec.DeepCopy()
	// don't worry about errors.  If we can't decode, we'll simply stomp over the field.
	existingConfig := map[string]interface{}{}
	json.NewDecoder(bytes.NewBuffer(spec.ObservedConfig.Raw)).Decode(&existingConfig)

	var errs []error
	var observedConfigs []map[string]interface{}
	for _, i := range rand.Perm(len(c.observers)) {
		var currErrs []error
		observedConfig, currErrs := c.observers[i](c.listers, existingConfig)
		observedConfigs = append(observedConfigs, observedConfig)
		errs = append(errs, currErrs...)
	}

	mergedObservedConfig := map[string]interface{}{}
	for _, observedConfig := range observedConfigs {
		mergo.Merge(&mergedObservedConfig, observedConfig)
	}

	if !equality.Semantic.DeepEqual(existingConfig, mergedObservedConfig) {
		glog.Infof("writing updated observedConfig: %v", diff.ObjectDiff(existingConfig, mergedObservedConfig))
		spec.ObservedConfig = runtime.RawExtension{Object: &unstructured.Unstructured{Object: mergedObservedConfig}}
		_, resourceVersion, err = c.operatorClient.UpdateOperatorSpec(resourceVersion, spec)
		if err != nil {
			errs = append(errs, fmt.Errorf("error writing updated observed config: %v", err))
		}
	}

	status := originalStatus.DeepCopy()
	if len(errs) > 0 {
		var messages []string
		for _, currentError := range errs {
			messages = append(messages, currentError.Error())
		}
		v1helpers.SetOperatorCondition(&status.Conditions, operatorv1.OperatorCondition{
			Type:    operatorStatusTypeConfigObservationFailing,
			Status:  operatorv1.ConditionTrue,
			Reason:  configObservationErrorConditionReason,
			Message: strings.Join(messages, "\n"),
		})

	} else {
		v1helpers.SetOperatorCondition(&status.Conditions, operatorv1.OperatorCondition{
			Type:   operatorStatusTypeConfigObservationFailing,
			Status: operatorv1.ConditionFalse,
		})
	}

	if !equality.Semantic.DeepEqual(originalStatus, status) {
		_, _, err = c.operatorClient.UpdateOperatorStatus(resourceVersion, status)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ConfigObserver) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting ConfigObserver")
	defer glog.Infof("Shutting down ConfigObserver")

	if !cache.WaitForCacheSync(stopCh, c.listers.PreRunHasSynced()...) {
		utilruntime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *ConfigObserver) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ConfigObserver) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	// before we call sync, we want to wait for token.  We do this to avoid hot looping.
	c.rateLimiter.Accept()

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// eventHandler queues the operator to check spec and status
func (c *ConfigObserver) EventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(configObserverWorkKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(configObserverWorkKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(configObserverWorkKey) },
	}
}
