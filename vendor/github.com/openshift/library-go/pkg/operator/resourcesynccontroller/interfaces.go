package resourcesynccontroller

import "k8s.io/apimachinery/pkg/util/sets"

// ResourceLocation describes coordinates for a resource to be synced
type ResourceLocation struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`

	// Provider if set for the source location enhance the error message to point to the component which
	// provide this resource.
	Provider string `json:"provider,omitempty"`
}

// PreconditionsFulfilled is a function that indicates whether all prerequisites
// are met and a resource can be synced.
type preconditionsFulfilled func() (bool, error)

func alwaysFulfilledPreconditions() (bool, error) { return true, nil }

type syncRuleSource struct {
	ResourceLocation
	syncedKeys               sets.Set[string]       // defines the set of keys to sync from source to dest
	preconditionsFulfilledFn preconditionsFulfilled // preconditions to fulfill before syncing the resource
}

type syncRules map[ResourceLocation]syncRuleSource

var (
	emptyResourceLocation = ResourceLocation{}
)

// ResourceSyncer allows changes to syncing rules by this controller
type ResourceSyncer interface {
	// SyncConfigMap indicates that a configmap should be copied from the source to the destination.  It will also
	// mirror a deletion from the source.  If the source is a zero object the destination will be deleted.
	SyncConfigMap(destination, source ResourceLocation) error
	// SyncSecret indicates that a secret should be copied from the source to the destination.  It will also
	// mirror a deletion from the source.  If the source is a zero object the destination will be deleted.
	SyncSecret(destination, source ResourceLocation) error
}
