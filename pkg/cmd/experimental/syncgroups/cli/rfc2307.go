package cli

import (
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/rfc2307"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

var _ SyncBuilder = &RFC2307SyncBuilder{}

type RFC2307SyncBuilder struct {
	ClientConfig *ldaputil.LDAPClientConfig
	Config       *api.RFC2307Config

	rfc2307LDAPInterface *rfc2307.LDAPInterface
}

func (b *RFC2307SyncBuilder) GetGroupLister() (interfaces.LDAPGroupLister, error) {
	return b.getRFC2307LDAPInterface()
}

func (b *RFC2307SyncBuilder) GetGroupNameMapper() (interfaces.LDAPGroupNameMapper, error) {
	ldapInterface, err := b.getRFC2307LDAPInterface()
	if err != nil {
		return nil, err
	}
	if b.Config.GroupNameAttributes != nil {
		return syncgroups.NewEntryAttributeGroupNameMapper(b.Config.GroupNameAttributes, ldapInterface), nil
	}

	return nil, nil
}

func (b *RFC2307SyncBuilder) GetUserNameMapper() (interfaces.LDAPUserNameMapper, error) {
	return syncgroups.NewUserNameMapper(b.Config.UserNameAttributes), nil
}

func (b *RFC2307SyncBuilder) GetGroupMemberExtractor() (interfaces.LDAPMemberExtractor, error) {
	return b.getRFC2307LDAPInterface()
}

func (b *RFC2307SyncBuilder) getRFC2307LDAPInterface() (*rfc2307.LDAPInterface, error) {
	if b.rfc2307LDAPInterface != nil {
		return b.rfc2307LDAPInterface, nil
	}

	groupQuery, err := ldaputil.NewLDAPQueryOnAttribute(b.Config.AllGroupsQuery, b.Config.GroupUIDAttribute)
	if err != nil {
		return nil, err
	}
	userQuery, err := ldaputil.NewLDAPQueryOnAttribute(b.Config.AllUsersQuery, b.Config.UserUIDAttribute)
	if err != nil {
		return nil, err
	}
	return rfc2307.NewLDAPInterface(b.ClientConfig,
		groupQuery, b.Config.GroupNameAttributes, b.Config.GroupMembershipAttributes,
		userQuery, b.Config.UserNameAttributes), nil
}
