package cli

import (
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/ldaputil/ldapclient"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/ad"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

var _ SyncBuilder = &ADBuilder{}

type ADBuilder struct {
	ClientConfig ldapclient.Config
	Config       *api.ActiveDirectoryConfig

	adLDAPInterface *ad.ADLDAPInterface
}

func (b *ADBuilder) GetGroupLister() (interfaces.LDAPGroupLister, error) {
	return b.getADLDAPInterface()
}

func (b *ADBuilder) GetGroupNameMapper() (interfaces.LDAPGroupNameMapper, error) {
	return &syncgroups.DNLDAPGroupNameMapper{}, nil
}

func (b *ADBuilder) GetUserNameMapper() (interfaces.LDAPUserNameMapper, error) {
	return syncgroups.NewUserNameMapper(b.Config.UserNameAttributes), nil
}

func (b *ADBuilder) GetGroupMemberExtractor() (interfaces.LDAPMemberExtractor, error) {
	return b.getADLDAPInterface()
}

func (b *ADBuilder) getADLDAPInterface() (*ad.ADLDAPInterface, error) {
	if b.adLDAPInterface != nil {
		return b.adLDAPInterface, nil
	}

	userQuery, err := ldaputil.NewLDAPQuery(b.Config.AllUsersQuery)
	if err != nil {
		return nil, err
	}
	return ad.NewADLDAPInterface(b.ClientConfig,
		userQuery, b.Config.GroupMembershipAttributes, b.Config.UserNameAttributes), nil
}
