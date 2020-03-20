package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/errors"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/encryption/controllers/migrators"
	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/secrets"
	"github.com/openshift/library-go/pkg/operator/encryption/state"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	// how long to wait until we retry a migration when it failed with unknown errors.
	migrationRetryDuration = time.Minute * 5
)

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
	component string
	name      string

	operatorClient operatorv1helpers.OperatorClient

	preRunCachesSynced []cache.InformerSynced

	encryptedGRs []schema.GroupResource

	encryptionSecretSelector metav1.ListOptions

	secretClient corev1client.SecretsGetter

	deployer statemachine.Deployer
	migrator migrators.Migrator
}

func NewMigrationController(
	component string,
	deployer statemachine.Deployer,
	migrator migrators.Migrator,
	operatorClient operatorv1helpers.OperatorClient,
	kubeInformersForNamespaces operatorv1helpers.KubeInformersForNamespaces,
	secretClient corev1client.SecretsGetter,
	encryptionSecretSelector metav1.ListOptions,
	eventRecorder events.Recorder,
	encryptedGRs []schema.GroupResource,
) factory.Controller {
	c := &migrationController{
		component:      component,
		name:           "EncryptionMigrationController",
		operatorClient: operatorClient,

		encryptedGRs: encryptedGRs,

		encryptionSecretSelector: encryptionSecretSelector,
		secretClient:             secretClient,
		deployer:                 deployer,
		migrator:                 migrator,
	}

	return factory.New().ResyncEvery(time.Second).WithSync(c.sync).WithInformers(
		migrator,
		operatorClient.Informer(),
		kubeInformersForNamespaces.InformersFor("openshift-config-managed").Core().V1().Secrets().Informer(),
		deployer,
	).ToController(c.name, eventRecorder.WithComponentSuffix("encryption-migration-controller"))
}

func (c *migrationController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	if ready, err := shouldRunEncryptionController(c.operatorClient); err != nil || !ready {
		return err // we will get re-kicked when the operator status updates
	}

	migratingResources, migrationError := c.migrateKeysIfNeededAndRevisionStable(syncCtx)

	// update failing condition
	degraded := operatorv1.OperatorCondition{
		Type:   "EncryptionMigrationControllerDegraded",
		Status: operatorv1.ConditionFalse,
	}
	if migrationError != nil {
		degraded.Status = operatorv1.ConditionTrue
		degraded.Reason = "Error"
		degraded.Message = migrationError.Error()
	}

	// update progressing condition
	progressing := operatorv1.OperatorCondition{
		Type:   "EncryptionMigrationControllerProgressing",
		Status: operatorv1.ConditionFalse,
	}
	if len(migratingResources) > 0 {
		progressing.Status = operatorv1.ConditionTrue
		progressing.Reason = "Migrating"
		progressing.Message = fmt.Sprintf("migrating resources to a new write key: %v", grsToHumanReadable(migratingResources))
	}

	if _, _, updateError := operatorv1helpers.UpdateStatus(c.operatorClient, operatorv1helpers.UpdateConditionFn(degraded), operatorv1helpers.UpdateConditionFn(progressing)); updateError != nil {
		return updateError
	}

	return migrationError
}

func (c *migrationController) setProgressing(migrating bool, reason, message string, args ...interface{}) error {
	// update progressing condition
	progressing := operatorv1.OperatorCondition{
		Type:    "EncryptionMigrationControllerProgressing",
		Status:  operatorv1.ConditionTrue,
		Reason:  reason,
		Message: fmt.Sprintf(message, args...),
	}
	if !migrating {
		progressing.Status = operatorv1.ConditionFalse
	}

	_, _, err := operatorv1helpers.UpdateStatus(c.operatorClient, operatorv1helpers.UpdateConditionFn(progressing))
	return err
}

// TODO doc
func (c *migrationController) migrateKeysIfNeededAndRevisionStable(syncContext factory.SyncContext) (migratingResources []schema.GroupResource, err error) {
	// no storage migration during revision changes
	currentEncryptionConfig, desiredEncryptionState, _, isTransitionalReason, err := statemachine.GetEncryptionConfigAndState(c.deployer, c.secretClient, c.encryptionSecretSelector, c.encryptedGRs)
	if err != nil {
		return nil, err
	}
	if currentEncryptionConfig == nil || len(isTransitionalReason) > 0 {
		syncContext.Queue().AddAfter(syncContext.QueueKey(), 2*time.Minute)
		return nil, nil
	}

	encryptionSecrets, err := secrets.ListKeySecrets(c.secretClient, c.encryptionSecretSelector)
	if err != nil {
		return nil, err
	}
	currentState, _ := encryptionconfig.ToEncryptionState(currentEncryptionConfig, encryptionSecrets)
	desiredEncryptedConfig := encryptionconfig.FromEncryptionState(desiredEncryptionState)

	// no storage migration until config is stable
	if !reflect.DeepEqual(currentEncryptionConfig.Resources, desiredEncryptedConfig.Resources) {
		// stop all running migrations
		for gr := range currentState {
			if err := c.migrator.PruneMigration(gr); err != nil {
				klog.Warningf("failed to interrupt migration for resource %s", gr)
				// ignore error
			}
		}

		syncContext.Queue().AddAfter(syncContext.QueueKey(), 2*time.Minute)
		return nil, nil // retry in a little while but do not go degraded
	}

	// sort by gr to get deterministic condition strings
	grs := []schema.GroupResource{}
	for gr := range currentState {
		grs = append(grs, gr)
	}
	sort.Slice(grs, func(i, j int) bool {
		return grs[i].String() < grs[j].String()
	})

	// all API servers have converged onto a single revision that matches our desired overall encryption state
	// now we know that it is safe to attempt key migrations
	// we never want to migrate during an intermediate state because that could lead to one API server
	// using a write key that another API server has not observed
	// this could lead to etcd storing data that not all API servers can decrypt
	var errs []error
	for _, gr := range grs {
		grActualKeys := currentState[gr]
		if !grActualKeys.HasWriteKey() {
			continue // no write key to migrate to
		}

		if alreadyMigrated, _, _ := state.MigratedFor([]schema.GroupResource{gr}, grActualKeys.WriteKey); alreadyMigrated {
			continue
		}

		// idem-potent migration start
		finished, result, when, err := c.migrator.EnsureMigration(gr, grActualKeys.WriteKey.Key.Name)
		if err == nil && finished && result != nil && time.Since(when) > migrationRetryDuration {
			// last migration error is far enough ago. Prune and retry.
			if err := c.migrator.PruneMigration(gr); err != nil {
				errs = append(errs, err)
				continue
			}
			finished, result, when, err = c.migrator.EnsureMigration(gr, grActualKeys.WriteKey.Key.Name)

		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if finished && result != nil {
			errs = append(errs, result)
			continue
		}

		if !finished {
			migratingResources = append(migratingResources, gr)
			continue
		}

		// update secret annotations
		oldWriteKey, err := secrets.FromKeyState(c.component, grActualKeys.WriteKey)
		if err != nil {
			errs = append(errs, result)
			continue
		}
		if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			s, err := c.secretClient.Secrets(oldWriteKey.Namespace).Get(context.TODO(), oldWriteKey.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get key secret %s/%s: %v", oldWriteKey.Namespace, oldWriteKey.Name, err)
			}

			changed, err := setResourceMigrated(gr, s)
			if !changed {
				return nil
			}

			_, _, updateErr := resourceapply.ApplySecret(c.secretClient, syncContext.Recorder(), s)
			return updateErr
		}); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	return migratingResources, errors.NewAggregate(errs)
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

// groupToHumanReadable extracts a group from gr and makes it more readable, for example it converts an empty group to "core"
// Note: do not use it to get resources from the server only when printing to a log file
func groupToHumanReadable(gr schema.GroupResource) string {
	group := gr.Group
	if len(group) == 0 {
		group = "core"
	}
	return group
}

func grsToHumanReadable(grs []schema.GroupResource) []string {
	ret := make([]string, 0, len(grs))
	for _, gr := range grs {
		ret = append(ret, fmt.Sprintf("%s/%s", groupToHumanReadable(gr), gr.Resource))
	}
	return ret
}
