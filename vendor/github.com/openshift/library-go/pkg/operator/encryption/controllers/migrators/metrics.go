package migrators

import (
	"github.com/prometheus/client_golang/prometheus"

	k8smetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

const (
	namespace = "storage_migrator"
	subsystem = "core_migrator"
)

// metrics provides access to all core migrator metrics.
var metrics *migratorMetrics

func init() {
	metrics = newMigratorMetrics(legacyregistry.Register)
}

// migratorMetrics instruments core migrator with prometheus metrics.
type migratorMetrics struct {
	objectsMigrated   *k8smetrics.CounterVec
	migration         *k8smetrics.CounterVec
	migrationDuration *k8smetrics.HistogramVec
}

// newMigratorMetrics create a new MigratorMetrics, configured with default metric names.
func newMigratorMetrics(registerFunc func(k8smetrics.Registerable) error) *migratorMetrics {
	// objectMigrates is defined in kube-storave-version-migrator
	objectsMigrated := k8smetrics.NewCounterVec(
		&k8smetrics.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "migrated_objects",
			Help:      "The total number of objects that have been migrated, labeled with the full resource name",
		}, []string{"resource"})
	registerFunc(objectsMigrated)

	// migration is defined in kube-storave-version-migrator
	migration := k8smetrics.NewCounterVec(
		&k8smetrics.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "migrations",
			Help:      "The total number of completed migration, labeled with the full resource name, and the status of the migration (failed or succeeded)",
		}, []string{"resource", "status"})
	registerFunc(migration)

	// migrationDuration is not defined upstream but uses the same Namespace and Subsystem
	// as the other metrics that are defined in kube-storave-version-migrator
	migrationDuration := k8smetrics.NewHistogramVec(
		&k8smetrics.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "migration_duration_seconds",
			Help:      "How long a successful migration takes in seconds, labeled with the full resource name",
			Buckets:   prometheus.ExponentialBuckets(120, 2, 7),
		}, []string{"resource"})
	registerFunc(migrationDuration)

	return &migratorMetrics{
		objectsMigrated:   objectsMigrated,
		migration:         migration,
		migrationDuration: migrationDuration,
	}
}

func (m *migratorMetrics) Reset() {
	m.objectsMigrated.Reset()
	m.migration.Reset()
}

// ObserveObjectsMigrated adds the number of migrated objects for a resource type
func (m *migratorMetrics) ObserveObjectsMigrated(added int, resource string) {
	m.objectsMigrated.WithLabelValues(resource).Add(float64(added))
}

// ObserveSucceededMigration increments the number of successful migrations for a resource type
func (m *migratorMetrics) ObserveSucceededMigration(resource string) {
	m.migration.WithLabelValues(resource, "Succeeded").Add(float64(1))
}

// ObserveFailedMigration increments the number of failed migrations for a resource type
func (m *migratorMetrics) ObserveFailedMigration(resource string) {
	m.migration.WithLabelValues(resource, "Failed").Add(float64(1))
}

// ObserveMigrationDuration records migration duration in seconds for a resource type
func (m *migratorMetrics) ObserveSucceededMigrationDuration(seconds float64, resource string) {
	m.migrationDuration.WithLabelValues(resource).Observe(seconds)
}
