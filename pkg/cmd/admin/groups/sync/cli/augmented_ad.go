package cli

import (
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/ad"
	"github.com/openshift/origin/pkg/cmd/admin/groups/sync/interfaces"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

var _ SyncBuilder = &AugmentedADBuilder{}
var _ PruneBuilder = &AugmentedADBuilder{}

type AugmentedADBuilder struct {
	ClientConfig ldapclient.Config
	Config       *api.AugmentedActiveDirectoryConfig

	augmentedADLDAPInterface *ad.AugmentedADLDAPInterface
}

func (b *AugmentedADBuilder) GetGroupLister() (interfaces.LDAPGroupLister, error) {
	return b.getAugmentedADLDAPInterface()
}

func (b *AugmentedADBuilder) GetGroupNameMapper() (interfaces.LDAPGroupNameMapper, error) {
	ldapInterface, err := b.getAugmentedADLDAPInterface()
	if err != nil {
		return nil, err
	}
	if b.Config.GroupNameAttributes != nil {
		return syncgroups.NewEntryAttributeGroupNameMapper(b.Config.GroupNameAttributes, ldapInterface), nil
	}

	return nil, nil
}

func (b *AugmentedADBuilder) GetUserNameMapper() (interfaces.LDAPUserNameMapper, error) {
	return syncgroups.NewUserNameMapper(b.Config.UserNameAttributes), nil
}

func (b *AugmentedADBuilder) GetGroupMemberExtractor() (interfaces.LDAPMemberExtractor, error) {
	return b.getAugmentedADLDAPInterface()
}

func (b *AugmentedADBuilder) getAugmentedADLDAPInterface() (*ad.AugmentedADLDAPInterface, error) {
	if b.augmentedADLDAPInterface != nil {
		return b.augmentedADLDAPInterface, nil
	}

	userQuery, err := ldaputil.NewLDAPQuery(b.Config.AllUsersQuery)
	if err != nil {
		return nil, err
	}
	groupQuery, err := ldaputil.NewLDAPQueryOnAttribute(b.Config.AllGroupsQuery, b.Config.GroupUIDAttribute)
	if err != nil {
		return nil, err
	}
	b.augmentedADLDAPInterface = ad.NewAugmentedADLDAPInterface(b.ClientConfig,
		userQuery, b.Config.GroupMembershipAttributes, b.Config.UserNameAttributes,
		groupQuery, b.Config.GroupNameAttributes)
	return b.augmentedADLDAPInterface, nil
}

func (b *AugmentedADBuilder) GetGroupDetector() (interfaces.LDAPGroupDetector, error) {
	return b.getAugmentedADLDAPInterface()
}
