package resourcesynccontroller

import (
	"context"
	"encoding/json"
	"fmt"
	applyoperatorv1 "github.com/openshift/client-go/operator/applyconfigurations/operator/v1"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// ResourceSyncController is a controller that will copy source configmaps and secrets to their destinations.
// It will also mirror deletions by deleting destinations.
type ResourceSyncController struct {
	controllerInstanceName string
	// syncRuleLock is used to ensure we avoid races on changes to syncing rules
	syncRuleLock sync.RWMutex
	// configMapSyncRules is a map from destination location to source location
	configMapSyncRules syncRules
	// secretSyncRules is a map from destination location to source location
	secretSyncRules syncRules

	// knownNamespaces is the list of namespaces we are watching.
	knownNamespaces sets.Set[string]

	configMapGetter            corev1client.ConfigMapsGetter
	secretGetter               corev1client.SecretsGetter
	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces
	operatorConfigClient       v1helpers.OperatorClient

	runFn   func(ctx context.Context, workers int)
	syncCtx factory.SyncContext
}

var _ ResourceSyncer = &ResourceSyncController{}
var _ factory.Controller = &ResourceSyncController{}

// NewResourceSyncController creates ResourceSyncController.
func NewResourceSyncController(
	instanceName string,
	operatorConfigClient v1helpers.OperatorClient,
	kubeInformersForNamespaces v1helpers.KubeInformersForNamespaces,
	secretsGetter corev1client.SecretsGetter,
	configMapsGetter corev1client.ConfigMapsGetter,
	eventRecorder events.Recorder,
) *ResourceSyncController {
	c := &ResourceSyncController{
		controllerInstanceName: factory.ControllerInstanceName(instanceName, "ResourceSync"),
		operatorConfigClient:   operatorConfigClient,

		configMapSyncRules:         syncRules{},
		secretSyncRules:            syncRules{},
		kubeInformersForNamespaces: kubeInformersForNamespaces,
		knownNamespaces:            kubeInformersForNamespaces.Namespaces(),

		configMapGetter: v1helpers.CachedConfigMapGetter(configMapsGetter, kubeInformersForNamespaces),
		secretGetter:    v1helpers.CachedSecretGetter(secretsGetter, kubeInformersForNamespaces),
		syncCtx:         factory.NewSyncContext("ResourceSyncController", eventRecorder.WithComponentSuffix("resource-sync-controller")),
	}

	informers := []factory.Informer{
		operatorConfigClient.Informer(),
	}
	for namespace := range kubeInformersForNamespaces.Namespaces() {
		if len(namespace) == 0 {
			continue
		}
		informer := kubeInformersForNamespaces.InformersFor(namespace)
		informers = append(informers, informer.Core().V1().ConfigMaps().Informer())
		informers = append(informers, informer.Core().V1().Secrets().Informer())
	}

	f := factory.New().
		WithSync(c.Sync).
		WithSyncContext(c.syncCtx).
		WithInformers(informers...).
		ResyncEvery(time.Minute).
		ToController(
			instanceName, // don't change what is passed here unless you also remove the old FooDegraded condition
			eventRecorder.WithComponentSuffix("resource-sync-controller"),
		)
	c.runFn = f.Run

	return c
}

func (c *ResourceSyncController) Run(ctx context.Context, workers int) {
	c.runFn(ctx, workers)
}

func (c *ResourceSyncController) Name() string {
	return c.controllerInstanceName
}

func (c *ResourceSyncController) SyncConfigMap(destination, source ResourceLocation) error {
	return c.syncConfigMap(destination, source, alwaysFulfilledPreconditions)
}

func (c *ResourceSyncController) SyncPartialConfigMap(destination ResourceLocation, source ResourceLocation, keys ...string) error {
	return c.syncConfigMap(destination, source, alwaysFulfilledPreconditions, keys...)
}

// SyncConfigMapConditionally adds a new configmap that the resource sync
// controller will synchronise if the given precondition is fulfilled.
func (c *ResourceSyncController) SyncConfigMapConditionally(destination, source ResourceLocation, preconditionsFulfilledFn preconditionsFulfilled) error {
	return c.syncConfigMap(destination, source, preconditionsFulfilledFn)
}

func (c *ResourceSyncController) syncConfigMap(destination ResourceLocation, source ResourceLocation, preconditionsFulfilledFn preconditionsFulfilled, keys ...string) error {
	if !c.knownNamespaces.Has(destination.Namespace) {
		return fmt.Errorf("not watching namespace %q", destination.Namespace)
	}
	if source != emptyResourceLocation && !c.knownNamespaces.Has(source.Namespace) {
		return fmt.Errorf("not watching namespace %q", source.Namespace)
	}

	c.syncRuleLock.Lock()
	defer c.syncRuleLock.Unlock()
	c.configMapSyncRules[destination] = syncRuleSource{
		ResourceLocation:         source,
		syncedKeys:               sets.New(keys...),
		preconditionsFulfilledFn: preconditionsFulfilledFn,
	}

	// make sure the new rule is picked up
	c.syncCtx.Queue().Add(c.syncCtx.QueueKey())
	return nil
}

func (c *ResourceSyncController) SyncSecret(destination, source ResourceLocation) error {
	return c.syncSecret(destination, source, alwaysFulfilledPreconditions)
}

func (c *ResourceSyncController) SyncPartialSecret(destination, source ResourceLocation, keys ...string) error {
	return c.syncSecret(destination, source, alwaysFulfilledPreconditions, keys...)
}

// SyncSecretConditionally adds a new secret that the resource sync controller
// will synchronise if the given precondition is fulfilled.
func (c *ResourceSyncController) SyncSecretConditionally(destination, source ResourceLocation, preconditionsFulfilledFn preconditionsFulfilled) error {
	return c.syncSecret(destination, source, preconditionsFulfilledFn)
}

func (c *ResourceSyncController) syncSecret(destination, source ResourceLocation, preconditionsFulfilledFn preconditionsFulfilled, keys ...string) error {
	if !c.knownNamespaces.Has(destination.Namespace) {
		return fmt.Errorf("not watching namespace %q", destination.Namespace)
	}
	if source != emptyResourceLocation && !c.knownNamespaces.Has(source.Namespace) {
		return fmt.Errorf("not watching namespace %q", source.Namespace)
	}

	c.syncRuleLock.Lock()
	defer c.syncRuleLock.Unlock()
	c.secretSyncRules[destination] = syncRuleSource{
		ResourceLocation:         source,
		syncedKeys:               sets.New(keys...),
		preconditionsFulfilledFn: preconditionsFulfilledFn,
	}

	// make sure the new rule is picked up
	c.syncCtx.Queue().Add(c.syncCtx.QueueKey())
	return nil
}

// errorWithProvider provides a finger of blame in case a source resource cannot be retrieved.
func errorWithProvider(provider string, err error) error {
	if len(provider) > 0 {
		return fmt.Errorf("%w (check the %q that is supposed to provide this resource)", err, provider)
	}
	return err
}

func (c *ResourceSyncController) Sync(ctx context.Context, syncCtx factory.SyncContext) error {
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
		// skip the sync if the preconditions aren't fulfilled
		if fulfilled, err := source.preconditionsFulfilledFn(); !fulfilled || err != nil {
			if err != nil {
				errors = append(errors, err)
			}
			continue
		}

		if source.ResourceLocation == emptyResourceLocation {
			// use the cache to check whether the configmap exists in target namespace, if not skip the extra delete call.
			if _, err := c.configMapGetter.ConfigMaps(destination.Namespace).Get(ctx, destination.Name, metav1.GetOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					errors = append(errors, err)
				}
				continue
			}
			if err := c.configMapGetter.ConfigMaps(destination.Namespace).Delete(ctx, destination.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				errors = append(errors, err)
			}
			continue
		}

		_, _, err := resourceapply.SyncPartialConfigMap(ctx, c.configMapGetter, syncCtx.Recorder(), source.Namespace, source.Name, destination.Namespace, destination.Name, source.syncedKeys, []metav1.OwnerReference{})
		if err != nil {
			errors = append(errors, errorWithProvider(source.Provider, err))
		}
	}
	for destination, source := range c.secretSyncRules {
		// skip the sync if the preconditions aren't fulfilled
		if fulfilled, err := source.preconditionsFulfilledFn(); !fulfilled || err != nil {
			if err != nil {
				errors = append(errors, err)
			}
			continue
		}

		if source.ResourceLocation == emptyResourceLocation {
			// use the cache to check whether the secret exists in target namespace, if not skip the extra delete call.
			if _, err := c.secretGetter.Secrets(destination.Namespace).Get(ctx, destination.Name, metav1.GetOptions{}); err != nil {
				if !apierrors.IsNotFound(err) {
					errors = append(errors, err)
				}
				continue
			}
			if err := c.secretGetter.Secrets(destination.Namespace).Delete(ctx, destination.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				errors = append(errors, err)
			}
			continue
		}

		_, _, err := resourceapply.SyncPartialSecret(ctx, c.secretGetter, syncCtx.Recorder(), source.Namespace, source.Name, destination.Namespace, destination.Name, source.syncedKeys, []metav1.OwnerReference{})
		if err != nil {
			errors = append(errors, errorWithProvider(source.Provider, err))
		}
	}

	if len(errors) > 0 {
		condition := applyoperatorv1.OperatorStatus().
			WithConditions(applyoperatorv1.OperatorCondition().
				WithType(condition.ResourceSyncControllerDegradedConditionType).
				WithStatus(operatorv1.ConditionTrue).
				WithReason("Error").
				WithMessage(v1helpers.NewMultiLineAggregate(errors).Error()))
		updateErr := c.operatorConfigClient.ApplyOperatorStatus(ctx, c.controllerInstanceName, condition)
		if updateErr != nil {
			return updateErr
		}
		return nil
	}

	condition := applyoperatorv1.OperatorStatus().
		WithConditions(applyoperatorv1.OperatorCondition().
			WithType(condition.ResourceSyncControllerDegradedConditionType).
			WithStatus(operatorv1.ConditionFalse))
	updateErr := c.operatorConfigClient.ApplyOperatorStatus(ctx, c.controllerInstanceName, condition)
	if updateErr != nil {
		return updateErr
	}
	return nil
}

func NewDebugHandler(controller *ResourceSyncController) http.Handler {
	return &debugHTTPHandler{controller: controller}
}

type debugHTTPHandler struct {
	controller *ResourceSyncController
}

type ResourceSyncRule struct {
	Destination ResourceLocation `json:"destination"`
	Source      syncRuleSource   `json:"source"`
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

func resourceSyncRuleList(syncRules syncRules) ResourceSyncRuleList {
	rules := make(ResourceSyncRuleList, 0, len(syncRules))
	for dest, src := range syncRules {
		rule := ResourceSyncRule{
			Source:      src,
			Destination: dest,
		}
		rules = append(rules, rule)
	}
	sort.Sort(rules)
	return rules
}
