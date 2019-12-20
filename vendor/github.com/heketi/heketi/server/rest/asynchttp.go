//
// Copyright (c) 2015 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package rest

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/heketi/heketi/pkg/idgen"
	"github.com/heketi/heketi/pkg/logging"
	"github.com/lpabon/godbc"
)

var (
	logger = logging.NewLogger("[asynchttp]", logging.LEVEL_INFO)
)

// Contains information about the asynchronous operation
type AsyncHttpHandler struct {
	err          error
	completed    bool
	manager      *AsyncHttpManager
	location, id string
}

// Manager of asynchronous operations
type AsyncHttpManager struct {
	IdGen    func() string
	lock     sync.RWMutex
	route    string
	handlers map[string]*AsyncHttpHandler
}

// Creates a new manager
func NewAsyncHttpManager(route string) *AsyncHttpManager {
	return &AsyncHttpManager{
		route:    route,
		handlers: make(map[string]*AsyncHttpHandler),
		IdGen:    idgen.GenUUID,
	}
}

// Use to create a new asynchronous operation handler.
// Only use this function if you need to do every step by hand.
// It is recommended to use AsyncHttpRedirectFunc() instead
func (a *AsyncHttpManager) NewHandler() *AsyncHttpHandler {
	return a.NewHandlerWithId(a.NewId())
}

// NewHandlerWithId constructs and returns an AsyncHttpHandler with the
// given ID. Compare to NewHandler() which automatically generates its
// own ID.
func (a *AsyncHttpManager) NewHandlerWithId(id string) *AsyncHttpHandler {
	handler := &AsyncHttpHandler{
		manager: a,
		id:      id,
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	_, idPresent := a.handlers[handler.id]
	godbc.Require(!idPresent)
	a.handlers[handler.id] = handler

	return handler
}

// NewId returns a new string id for a handler. This string is not preserved
// internally and must be passed to another function to be used.
func (a *AsyncHttpManager) NewId() string {
	return a.IdGen()
}

// Create an asynchronous operation handler and return the appropiate
// information the caller.
// This function will call handlerfunc() in a new go routine, then
// return to the caller a HTTP status 202 setting up the `Location` header
// to point to the new asynchronous handler.
//
// If handlerfunc() returns failure, the asynchronous handler will return
// an http status of 500 and save the error string in the body.
// If handlerfunc() is successful and returns a location url path in "string",
// the asynchronous handler will return 303 (See Other) with the Location
// header set to the value returned in the string.
// If handlerfunc() is successful and returns an empty string, then the
// asynchronous handler will return 204 to the caller.
//
// Example:
//      package rest
//		import (
//			"github.com/gorilla/mux"
//          "github.com/heketi/rest"
//			"net/http"
//			"net/http/httptest"
//			"time"
//		)
//
//		// Setup asynchronous manager
//		route := "/x"
//		manager := rest.NewAsyncHttpManager(route)
//
//		// Setup the route
//		router := mux.NewRouter()
//	 	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
//		router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
//			w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
//			w.WriteHeader(http.StatusOK)
//			fmt.Fprint(w, "HelloWorld")
//		}).Methods("GET")
//
//		router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
//			manager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
//				time.Sleep(100 * time.Millisecond)
//				return "/result", nil
//			})
//		}).Methods("GET")
//
//		// Setup the server
//		ts := httptest.NewServer(router)
//		defer ts.Close()
//
func (a *AsyncHttpManager) AsyncHttpRedirectFunc(w http.ResponseWriter,
	r *http.Request,
	handlerfunc func() (string, error)) {

	handler := a.NewHandler()
	handler.handle(handlerfunc)
	http.Redirect(w, r, handler.Url(), http.StatusAccepted)
}

func (a *AsyncHttpManager) AsyncHttpRedirectUsing(w http.ResponseWriter,
	r *http.Request,
	id string,
	handlerfunc func() (string, error)) {

	handler := a.NewHandlerWithId(id)
	handler.handle(handlerfunc)
	http.Redirect(w, r, handler.Url(), http.StatusAccepted)
}

// Handler for asynchronous operation status
// Register this handler with a router like Gorilla Mux
//
// Returns the following HTTP status codes
// 		200 Operation is still pending
//		404 Id requested does not exist
//		500 Operation finished and has failed.  Body will be filled in with the
//			error in plain text.
//		303 Operation finished and has setup a new location to retreive data.
//		204 Operation finished and has no data to return
//
// Example:
//      package rest
//		import (
//			"github.com/gorilla/mux"
//          "github.com/heketi/rest"
//			"net/http"
//			"net/http/httptest"
//			"time"
//		)
//
//		// Setup asynchronous manager
//		route := "/x"
//		manager := rest.NewAsyncHttpManager(route)
//
//		// Setup the route
//		router := mux.NewRouter()
//	 	router.HandleFunc(route+"/{id:[A-Fa-f0-9]+}", manager.HandlerStatus).Methods("GET")
//
//		// Setup the server
//		ts := httptest.NewServer(router)
//		defer ts.Close()
//
func (a *AsyncHttpManager) HandlerStatus(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	vars := mux.Vars(r)
	id := vars["id"]

	a.lock.Lock()
	defer a.lock.Unlock()

	// Check the id is in the map
	if handler, ok := a.handlers[id]; ok {

		if handler.completed {
			if handler.err != nil {

				// Return 500 status
				http.Error(w, handler.err.Error(), http.StatusInternalServerError)
			} else {
				if handler.location != "" {

					// Redirect to new location
					http.Redirect(w, r, handler.location, http.StatusSeeOther)
				} else {

					// Return 204 status
					w.WriteHeader(http.StatusNoContent)
				}
			}

			// It has been completed, we can now remove it from the map
			delete(a.handlers, id)
		} else {
			// Still pending
			// Could add a JSON body here later
			w.Header().Add("X-Pending", "true")
			w.WriteHeader(http.StatusOK)
		}

	} else {
		http.Error(w, "Id not found", http.StatusNotFound)
	}
}

// Returns the url for the specified asynchronous handler
func (h *AsyncHttpHandler) Url() string {
	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	return h.manager.route + "/" + h.id
}

// Registers that the handler has completed with an error
func (h *AsyncHttpHandler) CompletedWithError(err error) {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.err = err
	h.completed = true

	godbc.Ensure(h.completed == true)
}

// Registers that the handler has completed and has provided a location
// where information can be retreived
func (h *AsyncHttpHandler) CompletedWithLocation(location string) {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.location = location
	h.completed = true

	godbc.Ensure(h.completed == true)
	godbc.Ensure(h.location == location)
	godbc.Ensure(h.err == nil)
}

// Registers that the handler has completed and no data needs to be returned
func (h *AsyncHttpHandler) Completed() {

	h.manager.lock.RLock()
	defer h.manager.lock.RUnlock()

	godbc.Require(h.completed == false)

	h.completed = true

	godbc.Ensure(h.completed == true)
	godbc.Ensure(h.location == "")
	godbc.Ensure(h.err == nil)
}

// handle starts running the given function in the background (goroutine).
func (h *AsyncHttpHandler) handle(f func() (string, error)) {
	go func() {
		logger.Info("Started job %v", h.id)

		ts := time.Now()
		url, err := f()
		logger.Info("Completed job %v in %v", h.id, time.Since(ts))

		if err != nil {
			h.CompletedWithError(err)
		} else if url != "" {
			h.CompletedWithLocation(url)
		} else {
			h.Completed()
		}
	}()
}
