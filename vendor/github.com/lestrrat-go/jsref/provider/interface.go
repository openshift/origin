package provider

import (
	"net/http"
	"sync"
)

type FS struct {
	mp   *Map
	Root string
}

type HTTP struct {
	mp     *Map
	Client *http.Client
}

type Map struct {
	lock    sync.Mutex
	mapping map[string]interface{}
}
