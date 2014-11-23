package api

// TODO: Add display name to common meta?
type UserInfo interface {
	GetName() string
	GetUID() string
	GetScope() string
	GetExtra() map[string]string
}

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

// UserIdentityMapper maps UserIdentities into UserInfo objects to allow different user abstractions within auth code.
type UserIdentityMapper interface {
	// UserFor takes an identity, ignores the passed identity.Provider, forces the provider value to some other value and then creates the mapping.
	// It returns the corresponding UserInfo
	UserFor(identityInfo UserIdentityInfo) (UserInfo, error)
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

type DefaultUserInfo struct {
	Name  string
	UID   string
	Scope string
	Extra map[string]string
}

func (i *DefaultUserInfo) GetName() string {
	return i.Name
}

func (i *DefaultUserInfo) GetUID() string {
	return i.UID
}

func (i *DefaultUserInfo) GetScope() string {
	return i.Scope
}

func (i *DefaultUserInfo) GetExtra() map[string]string {
	return i.Extra
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
