package resourcesynccontroller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const controllerWorkQueueKey = "key"

// ResourceSyncController is a controller that will copy source configmaps and secrets to their destinations.
// It will also mirror deletions by deleting destinations.
type ResourceSyncController struct {
	// syncRuleLock is used to ensure we avoid races on changes to syncing rules
	syncRuleLock sync.RWMutex
	// configMapSyncRules is a map from destination location to source location
	configMapSyncRules map[ResourceLocation]ResourceLocation
	// secretSyncRules is a map from destination location to source location
	secretSyncRules map[ResourceLocation]ResourceLocation

	// knownNamespaces is the list of namespaces we are watching.
	knownNamespaces sets.String

	configMapGetter            corev1client.ConfigMapsGetter
	secretGetter               corev1client.SecretsGetter
	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces
	operatorConfigClient       v1helpers.OperatorClient

	cachesToSync  []cache.InformerSynced
	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder
}

var _ ResourceSyncer = &ResourceSyncController{}

// NewResourceSyncController creates ResourceSyncController.
func NewResourceSyncController(
	operatorConfigClient v1helpers.OperatorClient,
	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces,
	secretsGetter corev1client.SecretsGetter,
	configMapsGetter corev1client.ConfigMapsGetter,
	eventRecorder events.Recorder,
) *ResourceSyncController {
	c := &ResourceSyncController{
		operatorConfigClient: operatorConfigClient,
		eventRecorder:        eventRecorder.WithComponentSuffix("resource-sync-controller"),

		configMapSyncRules:         map[ResourceLocation]ResourceLocation{},
		secretSyncRules:            map[ResourceLocation]ResourceLocation{},
		kubeInformersForNamespaces: kubeInformersForNamespaces,
		knownNamespaces:            kubeInformersForNamespaces.Namespaces(),

		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ResourceSyncController"),
		configMapGetter: configMapsGetter,
		secretGetter:    secretsGetter,
	}

	for namespace := range kubeInformersForNamespaces.Namespaces() {
		if len(namespace) == 0 {
			continue
		}
		informers := kubeInformersForNamespaces.InformersFor(namespace)
		informers.Core().V1().ConfigMaps().Informer().AddEventHandler(c.eventHandler())
		informers.Core().V1().Secrets().Informer().AddEventHandler(c.eventHandler())

		c.cachesToSync = append(c.cachesToSync, informers.Core().V1().ConfigMaps().Informer().HasSynced)
		c.cachesToSync = append(c.cachesToSync, informers.Core().V1().Secrets().Informer().HasSynced)
	}

	// we watch this just in case someone messes with our status
	operatorConfigClient.Informer().AddEventHandler(c.eventHandler())

	c.cachesToSync = append(c.cachesToSync, operatorConfigClient.Informer().HasSynced)

	return c
}

func (c *ResourceSyncController) SyncConfigMap(destination, source ResourceLocation) error {
	if !c.knownNamespaces.Has(destination.Namespace) {
		return fmt.Errorf("not watching namespace %q", destination.Namespace)
	}
	if source != emptyResourceLocation && !c.knownNamespaces.Has(source.Namespace) {
		return fmt.Errorf("not watching namespace %q", source.Namespace)
	}

	c.syncRuleLock.Lock()
	defer c.syncRuleLock.Unlock()
	c.configMapSyncRules[destination] = source

	// make sure the new rule is picked up
	c.queue.Add(controllerWorkQueueKey)
	return nil
}

func (c *ResourceSyncController) SyncSecret(destination, source ResourceLocation) error {
	if !c.knownNamespaces.Has(destination.Namespace) {
		return fmt.Errorf("not watching namespace %q", destination.Namespace)
	}
	if source != emptyResourceLocation && !c.knownNamespaces.Has(source.Namespace) {
		return fmt.Errorf("not watching namespace %q", source.Namespace)
	}

	c.syncRuleLock.Lock()
	defer c.syncRuleLock.Unlock()
	c.secretSyncRules[destination] = source

	// make sure the new rule is picked up
	c.queue.Add(controllerWorkQueueKey)
	return nil
}

func (c *ResourceSyncController) sync() error {
	operatorSpec, _, _, err := c.operatorConfigClient.GetOperatorState()
	if err != nil {
		return err
	}

	if !management.IsOperatorManaged(operatorSpec.ManagementState) {
		return nil
	}

	c.syncRuleLock.RLock()
	defer c.syncRuleLock.RUnlock()

	errors := []error{}

	for destination, source := range c.configMapSyncRules {
		if source == emptyResourceLocation {
			// use the cache to check whether the configmap exists in target namespace, if not skip the extra delete call.
			if _, err := c.configMapGetter.ConfigMaps(destination.Namespace).Get(destination.Name, metav1.GetOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					errors = append(errors, err)
				}
				continue
			}
			if err := c.configMapGetter.ConfigMaps(destination.Namespace).Delete(destination.Name, nil); err != nil && !apierrors.IsNotFound(err) {
				errors = append(errors, err)
			}
			continue
		}

		_, _, err := resourceapply.SyncConfigMap(c.configMapGetter, c.eventRecorder, source.Namespace, source.Name, destination.Namespace, destination.Name, []metav1.OwnerReference{})
		if err != nil {
			errors = append(errors, err)
		}
	}
	for destination, source := range c.secretSyncRules {
		if source == emptyResourceLocation {
			// use the cache to check whether the secret exists in target namespace, if not skip the extra delete call.
			if _, err := c.secretGetter.Secrets(destination.Namespace).Get(destination.Name, metav1.GetOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					errors = append(errors, err)
				}
				continue
			}
			if err := c.secretGetter.Secrets(destination.Namespace).Delete(destination.Name, nil); err != nil && !apierrors.IsNotFound(err) {
				errors = append(errors, err)
			}
			continue
		}

		_, _, err := resourceapply.SyncSecret(c.secretGetter, c.eventRecorder, source.Namespace, source.Name, destination.Namespace, destination.Name, []metav1.OwnerReference{})
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		cond := operatorv1.OperatorCondition{
			Type:    condition.ResourceSyncControllerDegradedConditionType,
			Status:  operatorv1.ConditionTrue,
			Reason:  "Error",
			Message: v1helpers.NewMultiLineAggregate(errors).Error(),
		}
		if _, _, updateError := v1helpers.UpdateStatus(c.operatorConfigClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
			return updateError
		}
		return nil
	}

	cond := operatorv1.OperatorCondition{
		Type:   condition.ResourceSyncControllerDegradedConditionType,
		Status: operatorv1.ConditionFalse,
	}
	if _, _, updateError := v1helpers.UpdateStatus(c.operatorConfigClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		return updateError
	}
	return nil
}

func (c *ResourceSyncController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting ResourceSyncController")
	defer klog.Infof("Shutting down ResourceSyncController")
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *ResourceSyncController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ResourceSyncController) processNextWorkItem() bool {
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
func (c *ResourceSyncController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}

func NewDebugHandler(controller *ResourceSyncController) http.Handler {
	return &debugHTTPHandler{controller: controller}
}

type debugHTTPHandler struct {
	controller *ResourceSyncController
}

type ResourceSyncRule struct {
	Source      ResourceLocation `json:"source"`
	Destination ResourceLocation `json:"destination"`
}

type ResourceSyncRuleList []ResourceSyncRule

func (l ResourceSyncRuleList) Len() int      { return len(l) }
func (l ResourceSyncRuleList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l ResourceSyncRuleList) Less(i, j int) bool {
	if strings.Compare(l[i].Source.Namespace, l[j].Source.Namespace) < 0 {
		return true
	}
	if strings.Compare(l[i].Source.Namespace, l[j].Source.Namespace) > 0 {
		return false
	}
	if strings.Compare(l[i].Source.Name, l[j].Source.Name) < 0 {
		return true
	}
	return false
}

type ControllerSyncRules struct {
	Secrets ResourceSyncRuleList `json:"secrets"`
	Configs ResourceSyncRuleList `json:"configs"`
}

// ServeSyncRules provides a handler function to return the sync rules of the controller
func (h *debugHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	syncRules := ControllerSyncRules{ResourceSyncRuleList{}, ResourceSyncRuleList{}}

	h.controller.syncRuleLock.RLock()
	defer h.controller.syncRuleLock.RUnlock()
	syncRules.Secrets = append(syncRules.Secrets, resourceSyncRuleList(h.controller.secretSyncRules)...)
	syncRules.Configs = append(syncRules.Configs, resourceSyncRuleList(h.controller.configMapSyncRules)...)

	data, err := json.Marshal(syncRules)
	if err != nil {
		w.Write([]byte(err.Error()))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(data)
	w.WriteHeader(http.StatusOK)
}

func resourceSyncRuleList(syncRules map[ResourceLocation]ResourceLocation) ResourceSyncRuleList {
	rules := make(ResourceSyncRuleList, 0, len(syncRules))
	for src, dest := range syncRules {
		rule := ResourceSyncRule{
			Source:      src,
			Destination: dest,
		}
		rules = append(rules, rule)
	}
	sort.Sort(rules)
	return rules
}
