package osinserver

import (
	"net/http"

	"github.com/RangelReale/osin"
)

// mux is an object that can register http handlers.
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// AuthorizeHandler populates an AuthorizeRequest or handles the request itself
type AuthorizeHandler interface {
	// HandleAuthorize does one of the following:
	// 1. populates an AuthorizeRequest (typically the Authorized and UserData fields) and returns false
	// 2. populates the error fields of a Response object and returns false
	// 3. returns an internal server error and returns false
	// 4. writes the response itself and returns true
	HandleAuthorize(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (handled bool, err error)
}

type AuthorizeHandlerFunc func(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error)

func (f AuthorizeHandlerFunc) HandleAuthorize(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
	return f(ar, resp, w)
}

type AuthorizeHandlers []AuthorizeHandler

func (all AuthorizeHandlers) HandleAuthorize(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
	for _, h := range all {
		if handled, err := h.HandleAuthorize(ar, resp, w); handled || err != nil {
			return handled, err
		}
	}
	return false, nil
}

// AccessHandler populates an AccessRequest
type AccessHandler interface {
	// HandleAccess populates an AccessRequest (typically the Authorized and UserData fields)
	HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter) error
}

type AccessHandlerFunc func(ar *osin.AccessRequest, w http.ResponseWriter) error

func (f AccessHandlerFunc) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter) error {
	return f(ar, w)
}

type AccessHandlers []AccessHandler

func (all AccessHandlers) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter) error {
	for _, h := range all {
		if err := h.HandleAccess(ar, w); err != nil {
			return err
		}
	}
	return nil
}

// ErrorHandler writes an error response
type ErrorHandler interface {
	// HandleError writes an error response
	HandleError(err error, w http.ResponseWriter, req *http.Request)
}
