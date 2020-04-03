package migrators

import (
	"context"
	"fmt"
	"time"

	migrationv1alpha1 "github.com/kubernetes-sigs/kube-storage-version-migrator/pkg/apis/migration/v1alpha1"
	kubemigratorclient "github.com/kubernetes-sigs/kube-storage-version-migrator/pkg/clients/clientset"
	migrationv1alpha1informer "github.com/kubernetes-sigs/kube-storage-version-migrator/pkg/clients/informer/migration/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/cache"
)

const writeKeyAnnotationKey = "encryption.apiserver.operator.openshift.io/write-key"

func NewKubeStorageVersionMigrator(client kubemigratorclient.Interface, informer migrationv1alpha1informer.Interface, discoveryClient discovery.ServerResourcesInterface) *KubeStorageVersionMigrator {
	return &KubeStorageVersionMigrator{
		discoveryClient: discoveryClient,
		client:          client,
		informer:        informer,
	}
}

// KubeStorageVersionMigrator runs migration through the kube-storage-version-migrator components,
// driven by CustomResources.
type KubeStorageVersionMigrator struct {
	discoveryClient discovery.ServerResourcesInterface
	client          kubemigratorclient.Interface
	informer        migrationv1alpha1informer.Interface
	cacheSynced     func() bool
}

func (m *KubeStorageVersionMigrator) AddEventHandler(handler cache.ResourceEventHandler) {
	informer := m.informer.StorageVersionMigrations().Informer()
	informer.AddEventHandler(handler)
	m.cacheSynced = informer.HasSynced
}

func (m *KubeStorageVersionMigrator) HasSynced() bool {
	return m.cacheSynced()
}

func (m *KubeStorageVersionMigrator) EnsureMigration(gr schema.GroupResource, writeKey string) (finished bool, result error, ts time.Time, err error) {
	name := migrationResourceName(gr)
	if migration, err := m.informer.StorageVersionMigrations().Lister().Get(name); err != nil && !errors.IsNotFound(err) {
		return false, nil, time.Time{}, err
	} else if err == nil && migration.Annotations[writeKeyAnnotationKey] == writeKey {
		for _, c := range migration.Status.Conditions {
			switch c.Type {
			case migrationv1alpha1.MigrationSucceeded:
				if c.Status == corev1.ConditionTrue {
					return true, nil, c.LastUpdateTime.Time, nil
				}
			case migrationv1alpha1.MigrationFailed:
				if c.Status == corev1.ConditionTrue {
					return true, fmt.Errorf("migration of %s for key %q failed: %s", gr, writeKey, c.Message), c.LastUpdateTime.Time, nil
				}
			}
		}
		return false, nil, time.Time{}, nil
	} else if err == nil {
		if err := m.client.MigrationV1alpha1().StorageVersionMigrations().Delete(context.TODO(), name, metav1.DeleteOptions{
			Preconditions: &metav1.Preconditions{ResourceVersion: &migration.ResourceVersion},
		}); err != nil && !errors.IsNotFound(err) {
			return false, nil, time.Time{}, err
		}
	}

	v, err := preferredResourceVersion(m.discoveryClient, gr)
	if err != nil {
		return false, nil, time.Time{}, err
	}

	_, err = m.client.MigrationV1alpha1().StorageVersionMigrations().Create(context.TODO(), &migrationv1alpha1.StorageVersionMigration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				writeKeyAnnotationKey: writeKey,
			},
		},
		Spec: migrationv1alpha1.StorageVersionMigrationSpec{
			Resource: migrationv1alpha1.GroupVersionResource{
				Group:    gr.Group,
				Version:  v,
				Resource: gr.Resource,
			},
		},
	}, metav1.CreateOptions{})

	return false, nil, time.Time{}, err
}

func (m *KubeStorageVersionMigrator) PruneMigration(gr schema.GroupResource) error {
	name := migrationResourceName(gr)
	if err := m.client.MigrationV1alpha1().StorageVersionMigrations().Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func migrationResourceName(gr schema.GroupResource) string {
	return fmt.Sprintf("encryption-migration-%s-%s", groupToHumanReadable(gr), gr.Resource)
}

func groupToHumanReadable(gr schema.GroupResource) string {
	group := gr.Group
	if len(group) == 0 {
		group = "core"
	}
	return group
}
