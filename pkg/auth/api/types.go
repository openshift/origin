package api

// TODO: Add display name to common meta?
type UserInfo interface {
	GetName() string
	GetUID() string
	GetScope() string
	GetExtra() map[string]string
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
