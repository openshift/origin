package unsupportedconfigoverridescontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	unsupportedConfigOverridesControllerUpgradeable = "UnsupportedConfigOverridesUpgradeable"
	controllerWorkQueueKey                          = "key"
)

// UnsupportedConfigOverridesController is a controller that will copy source configmaps and secrets to their destinations.
// It will also mirror deletions by deleting destinations.
type UnsupportedConfigOverridesController struct {
	preRunCachesSynced []cache.InformerSynced

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface

	operatorClient v1helpers.OperatorClient
	eventRecorder  events.Recorder
}

// NewUnsupportedConfigOverridesController creates UnsupportedConfigOverridesController.
func NewUnsupportedConfigOverridesController(
	operatorClient v1helpers.OperatorClient,
	eventRecorder events.Recorder,
) *UnsupportedConfigOverridesController {
	c := &UnsupportedConfigOverridesController{
		operatorClient: operatorClient,
		eventRecorder:  eventRecorder.WithComponentSuffix("unsupported-config-overrides-controller"),

		preRunCachesSynced: []cache.InformerSynced{
			operatorClient.Informer().HasSynced,
		},
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "UnsupportedConfigOverridesController"),
	}

	operatorClient.Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c *UnsupportedConfigOverridesController) sync() error {
	operatorSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	cond := operatorv1.OperatorCondition{
		Type:   unsupportedConfigOverridesControllerUpgradeable,
		Status: operatorv1.ConditionTrue,
		Reason: "NoUnsupportedConfigOverrides",
	}
	if len(operatorSpec.UnsupportedConfigOverrides.Raw) > 0 {
		cond.Status = operatorv1.ConditionFalse
		cond.Reason = "UnsupportedConfigOverridesSet"
		cond.Message = fmt.Sprintf("unsupportedConfigOverrides=%v", string(operatorSpec.UnsupportedConfigOverrides.Raw))

		// try to get a prettier message
		keys, err := keysSetInUnsupportedConfig(operatorSpec.UnsupportedConfigOverrides.Raw)
		if err == nil {
			cond.Message = fmt.Sprintf("setting: %v", keys.List())

		}
	}

	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		return updateError
	}
	return nil
}

func keysSetInUnsupportedConfig(configYaml []byte) (sets.String, error) {
	configJson, err := kyaml.ToJSON(configYaml)
	if err != nil {
		glog.Warning(err)
		// maybe it's just json
		configJson = configYaml
	}

	config := map[string]interface{}{}
	if err := json.NewDecoder(bytes.NewBuffer(configJson)).Decode(&config); err != nil {
		return nil, err
	}

	return keysSetInUnsupportedConfigMap([]string{}, config), nil
}

func keysSetInUnsupportedConfigMap(pathSoFar []string, config map[string]interface{}) sets.String {
	ret := sets.String{}

	for k, v := range config {
		currPath := append(pathSoFar, k)

		switch castV := v.(type) {
		case map[string]interface{}:
			ret.Insert(keysSetInUnsupportedConfigMap(currPath, castV).UnsortedList()...)
		case []interface{}:
			ret.Insert(keysSetInUnsupportedConfigSlice(currPath, castV).UnsortedList()...)
		default:
			ret.Insert(strings.Join(currPath, "."))
		}
	}

	return ret
}

func keysSetInUnsupportedConfigSlice(pathSoFar []string, config []interface{}) sets.String {
	ret := sets.String{}

	for index, v := range config {
		currPath := append(pathSoFar, fmt.Sprintf("%d", index))

		switch castV := v.(type) {
		case map[string]interface{}:
			ret.Insert(keysSetInUnsupportedConfigMap(currPath, castV).UnsortedList()...)
		case []interface{}:
			ret.Insert(keysSetInUnsupportedConfigSlice(currPath, castV).UnsortedList()...)
		default:
			ret.Insert(strings.Join(currPath, "."))
		}
	}

	return ret
}

func (c *UnsupportedConfigOverridesController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting UnsupportedConfigOverridesController")
	defer glog.Infof("Shutting down UnsupportedConfigOverridesController")
	if !cache.WaitForCacheSync(stopCh, c.preRunCachesSynced...) {
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *UnsupportedConfigOverridesController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *UnsupportedConfigOverridesController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

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
func (c *UnsupportedConfigOverridesController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}
