package provider

import (
	"net/url"

	"github.com/lestrrat-go/pdebug"
	"github.com/pkg/errors"
)

func NewMap() *Map {
	return &Map{
		mapping: make(map[string]interface{}),
	}
}

func (mp *Map) Set(key string, v interface{}) error {
	mp.lock.Lock()
	defer mp.lock.Unlock()

	mp.mapping[key] = v
	return nil
}

func (mp *Map) Get(key *url.URL) (res interface{}, err error) {
	if pdebug.Enabled {
		g := pdebug.Marker("Map.Get(%s)", key).BindError(&err)
		defer g.End()
	}

	mp.lock.Lock()
	defer mp.lock.Unlock()

	v, ok := mp.mapping[key.String()]
	if !ok {
		return nil, errors.New("not found")
	}

	return v, nil
}

func (mp *Map) Reset() error {
	mp.lock.Lock()
	defer mp.lock.Unlock()

	mp.mapping = make(map[string]interface{})
	return nil
}
