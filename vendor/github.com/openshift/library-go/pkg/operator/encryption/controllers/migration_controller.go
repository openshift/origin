package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/pager"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

const migrationWorkKey = "key"

// The migrationController controller migrates resources to a new write key
// and annotated the write key secret afterwards with the migrated GRs. It
//
// * watches pods and secrets in <operand-target-namespace>
// * watches secrets in openshift-config-manager
// * computes a new, desired encryption config from encryption-config-<revision>
//   and the existing keys in openshift-config-managed.
// * compares desired with current target config and stops when they differ
// * checks the write-key secret whether
//   - encryption.apiserver.operator.openshift.io/migrated-timestamp annotation
//     is missing or
//   - a write-key for a resource does not show up in the
//     encryption.apiserver.operator.openshift.io/migrated-resources And then
//     starts a migration job (currently in-place synchronously, soon with the upstream migration tool)
// * updates the encryption.apiserver.operator.openshift.io/migrated-timestamp and
//   encryption.apiserver.operator.openshift.io/migrated-resources annotations on the
//   current write-key secrets.
type migrationController struct {
	operatorClient operatorv1helpers.OperatorClient

	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder

	preRunCachesSynced []cache.InformerSynced

	encryptedGRs []schema.GroupResource

	encryptionSecretSelector metav1.ListOptions

	secretClient corev1client.SecretsGetter

	deployer statemachine.Deployer

	dynamicClient   dynamic.Interface
	discoveryClient discovery.ServerResourcesInterface
}

func NewMigrationController(
	deployer statemachine.Deployer,
	operatorClient operatorv1helpers.OperatorClient,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	secretClient corev1client.SecretsGetter,
	encryptionSecretSelector metav1.ListOptions,
	eventRecorder events.Recorder,
	encryptedGRs []schema.GroupResource,
	dynamicClient dynamic.Interface, // temporary hack
	discoveryClient discovery.ServerResourcesInterface,
) *migrationController {
	c := &migrationController{
		operatorClient: operatorClient,

		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EncryptionMigrationController"),
		eventRecorder: eventRecorder.WithComponentSuffix("encryption-migration-controller"),

		encryptedGRs: encryptedGRs,

		encryptionSecretSelector: encryptionSecretSelector,
		secretClient:             secretClient,
		deployer:                 deployer,
		dynamicClient:            dynamicClient,
		discoveryClient:          discoveryClient,
	}

	c.preRunCachesSynced = setUpInformers(deployer, operatorClient, kubeInformersForNamespaces, c.eventHandler())

	return c
}

func (c *migrationController) sync() error {
	if ready, err := shouldRunEncryptionController(c.operatorClient); err != nil || !ready {
		return err // we will get re-kicked when the operator status updates
	}

	resetProgressing, configError := c.migrateKeysIfNeededAndRevisionStable()

	// update failing condition
	degraded := operatorv1.OperatorCondition{
		Type:   "EncryptionMigrationControllerDegraded",
		Status: operatorv1.ConditionFalse,
	}
	if configError != nil {
		degraded.Status = operatorv1.ConditionTrue
		degraded.Reason = "Error"
		degraded.Message = configError.Error()
	}

	updateFuncs := []operatorv1helpers.UpdateStatusFunc{operatorv1helpers.UpdateConditionFn(degraded)}

	// reset progressing condition
	if resetProgressing {
		progressing := operatorv1.OperatorCondition{
			Type:   "EncryptionMigrationControllerProgressing",
			Status: operatorv1.ConditionFalse,
		}
		updateFuncs = append(updateFuncs, operatorv1helpers.UpdateConditionFn(progressing))
	}
	if _, _, updateError := operatorv1helpers.UpdateStatus(c.operatorClient, updateFuncs...); updateError != nil {
		return updateError
	}

	return configError
}

func (c *migrationController) setProgressing(reason, message string, args ...interface{}) error {
	// update progressing condition
	progressing := operatorv1.OperatorCondition{
		Type:    "EncryptionMigrationControllerProgressing",
		Status:  operatorv1.ConditionTrue,
		Reason:  reason,
		Message: fmt.Sprintf(message, args...),
	}

	_, _, err := operatorv1helpers.UpdateStatus(c.operatorClient, operatorv1helpers.UpdateConditionFn(progressing))
	return err
}

// TODO doc
func (c *migrationController) migrateKeysIfNeededAndRevisionStable() (resetProgressing bool, err error) {
	// no storage migration during revision changes
	currentEncryptionConfig, desiredEncryptionState, _, isTransitionalReason, err := statemachine.GetEncryptionConfigAndState(c.deployer, c.secretClient, c.encryptionSecretSelector, c.encryptedGRs)
	if err != nil {
		return false, err
	}
	if currentEncryptionConfig == nil || len(isTransitionalReason) > 0 {
		c.queue.AddAfter(migrationWorkKey, 2*time.Minute)
		return true, nil
	}

	// no storage migration until config is stable
	desiredEncryptedConfig := encryptionconfig.FromEncryptionState(desiredEncryptionState)
	if !reflect.DeepEqual(currentEncryptionConfig.Resources, desiredEncryptedConfig.Resources) {
		c.queue.AddAfter(migrationWorkKey, 2*time.Minute)
		return true, nil // retry in a little while but do not go degraded
	}

	// all API servers have converged onto a single revision that matches our desired overall encryption state
	// now we know that it is safe to attempt key migrations
	// we never want to migrate during an intermediate state because that could lead to one API server
	// using a write key that another API server has not observed
	// this could lead to etcd storing data that not all API servers can decrypt
	for gr, grActualKeys := range encryptionconfig.ToEncryptionState(currentEncryptionConfig) {
		if !grActualKeys.HasWriteKey() {
			continue // no write key to migrate to
		}

		writeSecret, err := findSecretForKeyWithClient(grActualKeys.WriteKey, c.secretClient, c.encryptionSecretSelector)
		if err != nil {
			return true, err
		}
		ok := writeSecret != nil
		if !ok { // make sure this is a fully observed write key
			klog.V(4).Infof("write key %s for group=%s resource=%s not fully observed", grActualKeys.WriteKey.Key.Name, groupToHumanReadable(gr), gr.Resource)
			continue
		}

		if needsMigration(writeSecret, gr) {
			// storage migration takes a long time so we expose that via a distinct status change
			if err := c.setProgressing(strings.Title(groupToHumanReadable(gr))+strings.Title(gr.Resource), "migrating resource %s.%s to new write key", groupToHumanReadable(gr), gr.Resource); err != nil {
				return false, err
			}

			if err := c.runStorageMigration(gr); err != nil {
				return false, err
			}

			// update secret annotations
			if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
				s, err := c.secretClient.Secrets(writeSecret.Namespace).Get(writeSecret.Name, metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get key secret %s/%s: %v", writeSecret.Namespace, writeSecret.Name, err)
				}

				changed, err := setResourceMigrated(gr, s)
				if !changed {
					return nil
				}

				_, _, updateErr := resourceapply.ApplySecret(c.secretClient, c.eventRecorder, s)
				return updateErr
			}); err != nil {
				return false, err
			}
		}
	}

	// if we reach this, all migration went fine and we can reset progressing condition
	return true, nil
}

func findSecretForKeyWithClient(key state.KeyState, secretClient corev1client.SecretsGetter, encryptionSecretSelector metav1.ListOptions) (*corev1.Secret, error) {
	if len(key.Key.Name) == 0 {
		return nil, nil
	}

	secretList, err := secretClient.Secrets("openshift-config-managed").List(encryptionSecretSelector)
	if err != nil {
		return nil, err
	}

	for _, secret := range secretList.Items {
		sKeyAndMode, err := secrets.ToKeyState(&secret)
		if err != nil {
			// invalid
			continue
		}
		if state.EqualKeyAndEqualID(&sKeyAndMode, &key) {
			return &secret, nil
		}
	}

	return nil, nil
}

func setResourceMigrated(gr schema.GroupResource, s *corev1.Secret) (bool, error) {
	migratedGRs := secrets.MigratedGroupResources{}
	if existing, found := s.Annotations[secrets.EncryptionSecretMigratedResources]; found {
		if err := json.Unmarshal([]byte(existing), &migratedGRs); err != nil {
			// ignore error and just start fresh, causing some more migration at worst
			migratedGRs = secrets.MigratedGroupResources{}
		}
	}

	alreadyMigrated := false
	for _, existingGR := range migratedGRs.Resources {
		if existingGR == gr {
			alreadyMigrated = true
			break
		}
	}

	// update timestamp, if missing or first migration of gr
	if _, found := s.Annotations[secrets.EncryptionSecretMigratedTimestamp]; found && alreadyMigrated {
		return false, nil
	}
	if s.Annotations == nil {
		s.Annotations = map[string]string{}
	}
	s.Annotations[secrets.EncryptionSecretMigratedTimestamp] = time.Now().Format(time.RFC3339)

	// update resource list
	if !alreadyMigrated {
		migratedGRs.Resources = append(migratedGRs.Resources, gr)
		bs, err := json.Marshal(migratedGRs)
		if err != nil {
			return false, fmt.Errorf("failed to marshal %s annotation value %#v for key secret %s/%s", secrets.EncryptionSecretMigratedResources, migratedGRs, s.Namespace, s.Name)
		}
		s.Annotations[secrets.EncryptionSecretMigratedResources] = string(bs)
	}

	return true, nil
}

func needsMigration(secret *corev1.Secret, gr schema.GroupResource) bool {
	ks, err := secrets.ToKeyState(secret)
	if err != nil {
		klog.Infof("invalid key secret %s/%s", secret.Namespace, secret.Name)
		return false
	}
	alreadyMigrated, _, _ := state.MigratedFor([]schema.GroupResource{gr}, ks)
	return !alreadyMigrated
}

func (c *migrationController) runStorageMigration(gr schema.GroupResource) error {
	version, err := c.getVersion(gr)
	if err != nil {
		return err
	}
	d := c.dynamicClient.Resource(gr.WithVersion(version))

	var errs []error

	listPager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		allResource, err := d.List(opts)
		if err != nil {
			return nil, err // TODO this can wedge on resource expired errors with large overall list
		}
		for _, obj := range allResource.Items { // TODO parallelize for-loop
			_, updateErr := d.Namespace(obj.GetNamespace()).Update(&obj, metav1.UpdateOptions{})
			errs = append(errs, updateErr)
		}
		allResource.Items = nil // do not accumulate items, this fakes the visitor pattern
		return allResource, nil // leave the rest of the list intact to preserve continue token
	}))

	listPager.FullListIfExpired = false // prevent memory explosion from full list
	_, listErr := listPager.List(context.TODO(), metav1.ListOptions{})
	errs = append(errs, listErr)

	return utilerrors.FilterOut(utilerrors.NewAggregate(errs), errors.IsNotFound, errors.IsConflict)
}

func (c *migrationController) getVersion(gr schema.GroupResource) (string, error) {
	resourceLists, discoveryErr := c.discoveryClient.ServerPreferredResources() // safe to ignore error
	for _, resourceList := range resourceLists {
		groupVersion, err := schema.ParseGroupVersion(resourceList.GroupVersion)
		if err != nil {
			return "", err
		}
		if groupVersion.Group != gr.Group {
			continue
		}
		for _, resource := range resourceList.APIResources {
			if (len(resource.Group) == 0 || resource.Group == gr.Group) && resource.Name == gr.Resource {
				if len(resource.Version) > 0 {
					return resource.Version, nil
				}
				return groupVersion.Version, nil
			}
		}
	}
	return "", fmt.Errorf("failed to find version for %s, discoveryErr=%v", gr, discoveryErr)
}

func (c *migrationController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting EncryptionMigrationController")
	defer klog.Infof("Shutting down EncryptionMigrationController")
	if !cache.WaitForCacheSync(stopCh, c.preRunCachesSynced...) {
		utilruntime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}

	// only start one worker
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *migrationController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *migrationController) processNextWorkItem() bool {
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

	utilruntime.HandleError(fmt.Errorf("%v failed with: %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

func (c *migrationController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(migrationWorkKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(migrationWorkKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(migrationWorkKey) },
	}
}

// groupToHumanReadable extracts a group from gr and makes it more readable, for example it converts an empty group to "core"
// Note: do not use it to get resources from the server only when printing to a log file
func groupToHumanReadable(gr schema.GroupResource) string {
	group := gr.Group
	if len(group) == 0 {
		group = "core"
	}
	return group
}
