package teststorage

import (
	"errors"

	"github.com/RangelReale/osin"
)

type Test struct {
	Clients       map[string]osin.Client
	AuthorizeData *osin.AuthorizeData
	Authorize     map[string]*osin.AuthorizeData
	AccessData    *osin.AccessData
	Access        map[string]*osin.AccessData
	Err           error
}

func New() *Test {
	return &Test{
		Clients:   make(map[string]osin.Client),
		Authorize: make(map[string]*osin.AuthorizeData),
		Access:    make(map[string]*osin.AccessData),
	}
}

func (t *Test) Clone() osin.Storage {
	return t
}

func (t *Test) Close() {
}

// GetClient loads the client by id (client_id)
func (t *Test) GetClient(id string) (osin.Client, error) {
	return t.Clients[id], t.Err
}

// SaveAuthorize saves authorize data.
func (t *Test) SaveAuthorize(data *osin.AuthorizeData) error {
	t.AuthorizeData = data
	t.Authorize[data.Code] = data
	return t.Err
}

// LoadAuthorize looks up AuthorizeData by a code.
// Client information MUST be loaded together.
// Optionally can return error if expired.
func (t *Test) LoadAuthorize(code string) (*osin.AuthorizeData, error) {
	return t.Authorize[code], t.Err
}

// RemoveAuthorize revokes or deletes the authorization code.
func (t *Test) RemoveAuthorize(code string) error {
	delete(t.Authorize, code)
	return t.Err
}

// SaveAccess writes AccessData.
// If RefreshToken is not blank, it must save in a way that can be loaded using LoadRefresh.
func (t *Test) SaveAccess(data *osin.AccessData) error {
	t.AccessData = data
	t.Access[data.AccessToken] = data
	return t.Err
}

// LoadAccess retrieves access data by token. Client information MUST be loaded together.
// AuthorizeData and AccessData DON'T NEED to be loaded if not easily available.
// Optionally can return error if expired.
func (t *Test) LoadAccess(token string) (*osin.AccessData, error) {
	return t.Access[token], t.Err
}

// RemoveAccess revokes or deletes an AccessData.
func (t *Test) RemoveAccess(token string) error {
	delete(t.Access, token)
	return t.Err
}

// LoadRefresh retrieves refresh AccessData. Client information MUST be loaded together.
// AuthorizeData and AccessData DON'T NEED to be loaded if not easily available.
// Optionally can return error if expired.
func (t *Test) LoadRefresh(token string) (*osin.AccessData, error) {
	for _, v := range t.Access {
		if v.RefreshToken == token {
			return v, t.Err
		}
	}
	return nil, errors.New("not found")
}

// RemoveRefresh revokes or deletes refresh AccessData.
func (t *Test) RemoveRefresh(token string) error {
	data, _ := t.LoadRefresh(token)
	if data != nil {
		delete(t.Access, data.AccessToken)
	}
	return t.Err
}
