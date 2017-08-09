package dockerproxy

import (
	"net/http"
)

type Interceptor interface {
	InterceptRequest(*http.Request) error
	InterceptResponse(*http.Response) error
}

var Allow Interceptor = allow{}

type allow struct{}

func (allow) InterceptRequest(*http.Request) error   { return nil }
func (allow) InterceptResponse(*http.Response) error { return nil }
