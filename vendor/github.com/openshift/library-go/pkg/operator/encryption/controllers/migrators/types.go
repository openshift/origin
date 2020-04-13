package migrators

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/openshift/library-go/pkg/controller/factory"
)

// Migrator is a resource migration mechanism.
type Migrator interface {
	// EnsureMigration starts a migration if it does not exist. If a migration of
	// the same write-key exists and is finished (with or without error), nothing happens.
	// If a migration of another key exists, that migration is deleted first before
	// starting a new one. This function is idem-potent as long as a running or finished
	// migration is not pruned.
	// If finished is true, result is the result of the migration, with nil meaning that it
	// finished successfully. The timestamp shows when it has been finished.
	EnsureMigration(gr schema.GroupResource, writeKey string) (finished bool, result error, ts time.Time, err error)
	// PruneMigration removes a migration, independently whether it is running or finished,
	// with error or not. If there is no migration, this must not return an error.
	PruneMigration(gr schema.GroupResource) error

	factory.Informer
}
