package resourcesynccontroller

// ResourceLocation describes coordinates for a resource to be synced
type ResourceLocation struct {
	Namespace string
	Name      string
}

var emptyResourceLocation = ResourceLocation{}

// ResourceSyncer allows changes to syncing rules by this controller
type ResourceSyncer interface {
	// SyncConfigMap indicates that a configmap should be copied from the source to the destination.  It will also
	// mirror a deletion from the source.  If the source is a zero object the destination will be deleted.
	SyncConfigMap(destination, source ResourceLocation) error
	// SyncSecret indicates that a secret should be copied from the source to the destination.  It will also
	// mirror a deletion from the source.  If the source is a zero object the destination will be deleted.
	SyncSecret(destination, source ResourceLocation) error
}
