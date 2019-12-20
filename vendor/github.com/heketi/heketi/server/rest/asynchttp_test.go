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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/heketi/tests"
)

func TestNewManager(t *testing.T) {

	manager := NewAsyncHttpManager("/x")

	tests.Assert(t, len(manager.handlers) == 0)
	tests.Assert(t, manager.route == "/x")

}

func TestNewHandler(t *testing.T) {

	manager := NewAsyncHttpManager("/x")

	handler := manager.NewHandler()
	tests.Assert(t, handler.location == "")
	tests.Assert(t, handler.id != "")
	tests.Assert(t, handler.completed == false)
	tests.Assert(t, handler.err == nil)
	tests.Assert(t, manager.handlers[handler.id] == handler)
}

func TestHandlerUrl(t *testing.T) {
	manager := NewAsyncHttpManager("/x")
	handler := manager.NewHandler()

	// overwrite id value
	handler.id = "12345"
	tests.Assert(t, handler.Url() == "/x/12345")
}

func TestHandlerNotFound(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Request
	r, err := http.Get(ts.URL + route + "/12345")
	tests.Assert(t, r.StatusCode == http.StatusNotFound)
	tests.Assert(t, err == nil)
}

func TestHandlerCompletions(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)
	handler := manager.NewHandler()

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Test", "HelloWorld")
		w.WriteHeader(http.StatusOK)
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Request
	r, err := http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)

	// Handler completion without data
	handler.Completed()
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusNoContent)
	tests.Assert(t, err == nil)

	// Check that it was removed from the map
	_, ok := manager.handlers[handler.id]
	tests.Assert(t, ok == false)

	// Create new handler
	handler = manager.NewHandler()

	// Request
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)

	// Complete with error
	error_string := "This is a test"
	handler.CompletedWithError(errors.New(error_string))
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
	tests.Assert(t, err == nil)

	// Check body has error string
	body, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	tests.Assert(t, string(body) == error_string+"\n")

	// Create new handler
	handler = manager.NewHandler()

	// Request
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, err == nil)

	// Complete with SeeOther to Location
	handler.CompletedWithLocation("/test")

	// http.Get() looks at the Location header
	// and automatically redirects to the new location
	r, err = http.Get(ts.URL + handler.Url())
	tests.Assert(t, r.StatusCode == http.StatusOK)
	tests.Assert(t, r.Header.Get("X-Test") == "HelloWorld")
	tests.Assert(t, err == nil)

}

func TestAsyncHttpRedirectFunc(t *testing.T) {
	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")

	// Start testing error condition
	handlerfunc := func() (string, error) {
		return "", errors.New("Test Handler Function")
	}

	// The variable 'handlerfunc' can be changed outside the scope to
	// point to a new function.  Isn't Go awesome!
	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		manager.AsyncHttpRedirectFunc(w, r, handlerfunc)
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Get /app url
	r, err := http.Get(ts.URL + "/app")
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	tests.Assert(t, err == nil)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	// Expect the error
	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") != "true" {
			tests.Assert(t, r.StatusCode == http.StatusInternalServerError)
			body, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			tests.Assert(t, err == nil)
			tests.Assert(t, string(body) == "Test Handler Function\n")
			break
		} else {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond)
		}
	}

	// Set handler function to return a url to /result
	handlerfunc = func() (string, error) {
		return "/result", nil
	}

	// Get /app url
	r, err = http.Get(ts.URL + "/app")
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	tests.Assert(t, err == nil)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Should have the content from /result.  http.Get() automatically
	// retreives the content when a status of SeeOther is set and the
	// Location header has the next URL.
	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") != "true" {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			body, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			tests.Assert(t, err == nil)
			tests.Assert(t, string(body) == "HelloWorld")
			break
		} else {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond)
		}
	}

	// Test no redirect, simple completion
	handlerfunc = func() (string, error) {
		return "", nil
	}

	// Get /app url
	r, err = http.Get(ts.URL + "/app")
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	tests.Assert(t, err == nil)
	location, err = r.Location()
	tests.Assert(t, err == nil)

	// Should be success
	for {
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		if r.Header.Get("X-Pending") != "true" {
			tests.Assert(t, r.StatusCode == http.StatusNoContent)
			tests.Assert(t, r.ContentLength == 0)
			break
		} else {
			tests.Assert(t, r.StatusCode == http.StatusOK)
			time.Sleep(time.Millisecond)
		}
	}

}

func TestHandlerConcurrency(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	var wg sync.WaitGroup
	errorsch := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handler := manager.NewHandler()
			go func() {
				time.Sleep(10 * time.Millisecond)
				handler.Completed()
			}()

			for {
				r, err := http.Get(ts.URL + handler.Url())
				if err != nil {
					errorsch <- errors.New("Unable to get data from handler")
					return
				}
				if r.StatusCode == http.StatusNoContent {
					return
				} else if r.StatusCode == http.StatusOK {
					time.Sleep(time.Millisecond)
				} else {
					errorsch <- errors.New(fmt.Sprintf("Bad status returned: %d\n", r.StatusCode))
					return
				}
			}
		}()
	}
	wg.Wait()
	tests.Assert(t, len(errorsch) == 0)
}

func TestHandlerApplication(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")
	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		handler := manager.NewHandler()
		go func() {
			time.Sleep(500 * time.Millisecond)
			handler.CompletedWithLocation("/result")
		}()

		http.Redirect(w, r, handler.Url(), http.StatusAccepted)
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Get /app url
	r, err := http.Get(ts.URL + "/app")
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	tests.Assert(t, err == nil)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	for {
		// Since Get automatically redirects, we will
		// just keep asking until we get a body
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.ContentLength > 0 {
			body, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			tests.Assert(t, err == nil)
			tests.Assert(t, string(body) == "HelloWorld")
			break
		} else {
			tests.Assert(t, r.Header.Get("X-Pending") == "true")
			time.Sleep(time.Millisecond)
		}
	}

}

func TestApplicationWithRedirectFunc(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")

	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		manager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
			time.Sleep(500 * time.Millisecond)
			return "/result", nil
		})
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Get /app url
	r, err := http.Get(ts.URL + "/app")
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	tests.Assert(t, err == nil)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	for {
		// Since Get automatically redirects, we will
		// just keep asking until we get a body
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.ContentLength > 0 {
			body, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			tests.Assert(t, err == nil)
			tests.Assert(t, string(body) == "HelloWorld")
			break
		} else {
			tests.Assert(t, r.Header.Get("X-Pending") == "true")
			time.Sleep(time.Millisecond)
		}
	}

}

func TestAsyncHttpRedirectUsing(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")

	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		manager.AsyncHttpRedirectUsing(w, r, "bob", func() (string, error) {
			time.Sleep(500 * time.Millisecond)
			return "/result", nil
		})
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	r, err := http.Get(ts.URL + "/app")
	tests.Assert(t, r.StatusCode == http.StatusAccepted)
	tests.Assert(t, err == nil)
	location, err := r.Location()
	tests.Assert(t, err == nil)

	tests.Assert(t, strings.Contains(location.String(), "bob"),
		`expected "bob" in newloc, got:`, location)

	for {
		// Since Get automatically redirects, we will
		// just keep asking until we get a body
		r, err := http.Get(location.String())
		tests.Assert(t, err == nil)
		tests.Assert(t, r.StatusCode == http.StatusOK)
		if r.ContentLength > 0 {
			body, err := ioutil.ReadAll(r.Body)
			r.Body.Close()
			tests.Assert(t, err == nil)
			tests.Assert(t, string(body) == "HelloWorld")
			break
		} else {
			tests.Assert(t, r.Header.Get("X-Pending") == "true")
			time.Sleep(time.Millisecond)
		}
	}

}

func TestAsyncHttpRedirectFuncCustomIds(t *testing.T) {

	// Setup asynchronous manager
	route := "/x"
	manager := NewAsyncHttpManager(route)

	i := 0
	manager.IdGen = func() string {
		s := fmt.Sprintf("abc-%v%v%v", i, i, i)
		i += 1
		return s
	}

	// Setup the route
	router := mux.NewRouter()
	router.HandleFunc(route+"/{id}", manager.HandlerStatus).Methods("GET")
	router.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "HelloWorld")
	}).Methods("GET")

	router.HandleFunc("/app", func(w http.ResponseWriter, r *http.Request) {
		manager.AsyncHttpRedirectFunc(w, r, func() (string, error) {
			time.Sleep(500 * time.Millisecond)
			return "/result", nil
		})
	}).Methods("GET")

	// Setup the server
	ts := httptest.NewServer(router)
	defer ts.Close()

	for j := 0; j < 4; j++ {
		r, err := http.Get(ts.URL + "/app")
		tests.Assert(t, r.StatusCode == http.StatusAccepted)
		tests.Assert(t, err == nil)
		location, err := r.Location()
		tests.Assert(t, err == nil)

		// test that our custom id generator generated the IDs
		// according to our pattern
		tests.Assert(t, strings.Contains(location.String(), "abc-"),
			`expected "abc-" in newloc, got:`, location)
		part := fmt.Sprintf("-%v%v%v", j, j, j)
		tests.Assert(t, strings.Contains(location.String(), part),
			"expected", part, "in newloc, got:", location)

		for {
			// Since Get automatically redirects, we will
			// just keep asking until we get a body
			r, err := http.Get(location.String())
			tests.Assert(t, err == nil)
			tests.Assert(t, r.StatusCode == http.StatusOK)
			if r.ContentLength > 0 {
				body, err := ioutil.ReadAll(r.Body)
				r.Body.Close()
				tests.Assert(t, err == nil)
				tests.Assert(t, string(body) == "HelloWorld")
				break
			} else {
				tests.Assert(t, r.Header.Get("X-Pending") == "true")
				time.Sleep(time.Millisecond)
			}
		}
	}

}
