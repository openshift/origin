package usercontext

import (
	"net/http"
	"sync"

	"github.com/openshift/origin/pkg/authn/api"
)

var context map[*http.Request]interface{}
var lock sync.Mutex

func init() {
	context = make(map[*http.Request]interface{})
}

func set(req *http.Request, obj interface{}) {
	lock.Lock()
	defer lock.Unlock()
	context[req] = obj
}

func free(req *http.Request) {
	lock.Lock()
	defer lock.Unlock()
	delete(context, req)
}

func With(req *http.Request, obj interface{}, handle func()) {
	set(req, obj)
	defer free(req)
	handle()
}
