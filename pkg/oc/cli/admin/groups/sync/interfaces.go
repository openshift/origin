package sync

import (
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/origin/pkg/oc/lib/groupsync/interfaces"
)

// SyncBuilder describes an object that can build all the schema-specific parts of an LDAPGroupSyncer
type SyncBuilder interface {
	GetGroupLister() (interfaces.LDAPGroupLister, error)
	GetGroupNameMapper() (interfaces.LDAPGroupNameMapper, error)
	GetUserNameMapper() (interfaces.LDAPUserNameMapper, error)
	GetGroupMemberExtractor() (interfaces.LDAPMemberExtractor, error)
}

// PruneBuilder describes an object that can build all the schema-specific parts of an LDAPGroupPruner
type PruneBuilder interface {
	GetGroupLister() (interfaces.LDAPGroupLister, error)
	GetGroupNameMapper() (interfaces.LDAPGroupNameMapper, error)
	GetGroupDetector() (interfaces.LDAPGroupDetector, error)
}

// GroupNameRestrictions desribes an object that holds blacklists and whitelists
type GroupNameRestrictions interface {
	GetWhitelist() []string
	GetBlacklist() []string
}

// OpenShiftGroupNameRestrictions describes an object that holds blacklists and whitelists as well as
// a client that can retrieve OpenShift groups to satisfy those lists
type OpenShiftGroupNameRestrictions interface {
	GroupNameRestrictions
	GetClient() userv1client.GroupInterface
}

// MappedNameRestrictions describes an object that holds user name mappings for a group sync job
type MappedNameRestrictions interface {
	GetGroupNameMappings() map[string]string
}
