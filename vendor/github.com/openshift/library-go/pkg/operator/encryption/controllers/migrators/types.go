package migrators

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// Migrator is a resource migration mechanism.
type Migrator interface {
	// EnsureMigration starts a migration if it does not exist. If a migration of
	// the same write-key exists and is finished (with or without error), nothing happens.
	// If a migration of another key exists, that migration is deleted first before
	// starting a new one. This function is idem-potent as long as a running or finished
	// migration is not pruned.
	// If finished is true, result is the result of the migration, with nil meaning that it
	// finished successfully.
	EnsureMigration(gr schema.GroupResource, writeKey string) (finished bool, result error, err error)
	// PruneMigration removes a migration, independently whether it is running or finished,
	// with error or not. If there is no migration, this must not return an error.
	PruneMigration(gr schema.GroupResource) error

	// AddEventHandler registers a event handler whenever the resources change
	// that might influence the result of Migrations().
	AddEventHandler(handler cache.ResourceEventHandler) []cache.InformerSynced
}
