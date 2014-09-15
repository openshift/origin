package context

import (
	"net/http"
	"sync"
)

type RequestContextMap struct {
	objs map[*http.Request]interface{}
	lock sync.Mutex
}

func NewRequestContextMap() *RequestContextMap {
	return &RequestContextMap{
		objs: make(map[*http.Request]interface{}),
	}
}

func (c *RequestContextMap) Set(req *http.Request, obj interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.objs[req] = obj
}

func (c *RequestContextMap) Remove(req *http.Request) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.objs, req)
}

func (c *RequestContextMap) Get(req *http.Request) (interface{}, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	obj, ok := c.objs[req]
	return obj, ok
}
