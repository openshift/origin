package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
)

// UserIdentityInfo contains information about an identity.  Identities are distinct from users.  An authentication server of
// some kind (like oauth for example) describes an identity.  Our system controls the users mapped to this identity.
type UserIdentityInfo interface {
	// GetUserName uniquely identifies this particular identity for this provider.  It is NOT guaranteed to be unique across providers
	GetUserName() string
	// GetProviderName returns the name of the provider of this identity.
	GetProviderName() string
	// GetExtra is a map to allow providers to add additional fields that they understand
	GetExtra() map[string]string
}

// UserIdentityMapper maps UserIdentities into user.Info objects to allow different user abstractions within auth code.
type UserIdentityMapper interface {
	// UserFor takes an identity, ignores the passed identity.Provider, forces the provider value to some other value and then creates the mapping.
	// It returns the corresponding user.Info
	UserFor(identityInfo UserIdentityInfo) (user.Info, error)
}

type Client interface {
	GetId() string
	GetSecret() string
	GetRedirectUri() string
	GetUserData() interface{}
}

type Grant struct {
	Client      Client
	Scope       string
	Expiration  int64
	RedirectURI string
}

type DefaultUserIdentityInfo struct {
	UserName     string
	ProviderName string
	Extra        map[string]string
}

// NewDefaultUserIdentityInfo returns a DefaultUserIdentity info with a non-nil Extra component
func NewDefaultUserIdentityInfo(username string) DefaultUserIdentityInfo {
	return DefaultUserIdentityInfo{
		UserName: username,
		Extra:    make(map[string]string),
	}
}

func (i *DefaultUserIdentityInfo) GetUserName() string {
	return i.UserName
}

func (i *DefaultUserIdentityInfo) GetProviderName() string {
	return i.ProviderName
}

func (i *DefaultUserIdentityInfo) GetExtra() map[string]string {
	return i.Extra
}
