package interceptor

import (
	"context"
	"fmt"
	"net/http"
)

type BuildAuthorizer interface {
	AuthorizeBuildRequest(ctx context.Context, build *BuildImageOptions, auth *AuthOptions) (*BuildImageOptions, error)
}

type Interface interface {
	InterceptRequest(*http.Request) error
	InterceptResponse(*http.Response) error
}

type Proxy interface {
	Intercept(Interface, http.ResponseWriter, *http.Request)
}

var Allow Interface = allow{}

type allow struct{}

func (allow) InterceptRequest(r *http.Request) error   { return nil }
func (allow) InterceptResponse(r *http.Response) error { return nil }

type ErrorHandler interface {
	error
	http.Handler
}

func NewForbiddenError(err error) ErrorHandler {
	return forbiddenError{err: err}
}

type forbiddenError struct {
	err error
}

func (e forbiddenError) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return "forbidden"
}

func (e forbiddenError) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e.err != nil {
		http.Error(w, fmt.Sprintf("This call is forbidden: %v", e.err), http.StatusForbidden)
	} else {
		http.Error(w, "This call is forbidden", http.StatusForbidden)
	}
}
