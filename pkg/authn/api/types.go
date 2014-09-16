package api

type UserInfo interface {
	GetName() string
	GetUID() string
	GetScope() string
	GetExtra() map[string]string
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
