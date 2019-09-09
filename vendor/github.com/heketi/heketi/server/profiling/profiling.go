//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.

package profiling

// Based on ideas from https://github.com/mistifyio/negroni-pprof

import (
	"fmt"
	"net/http"
	"net/http/pprof"
	"os"
	runtime_pprof "runtime/pprof"

	"github.com/gorilla/mux"
)

var (
	basePath string = "/debug/pprof/"
)

func addPath(router *mux.Router, name string, handler http.Handler) {
	router.Path(basePath + name).Name(name).Handler(handler)
	fmt.Fprintf(os.Stderr, "DEBUG: registered profiling handler on %s\n", basePath+name)
}

func EnableProfiling(router *mux.Router) {
	for _, profile := range runtime_pprof.Profiles() {
		name := profile.Name()
		handler := pprof.Handler(name)

		addPath(router, name, handler)
	}

	// static profiles as listed in net/http/pprof/pprof.go:init()
	addPath(router, "cmdline", http.HandlerFunc(pprof.Cmdline))
	addPath(router, "profile", http.HandlerFunc(pprof.Profile))
	addPath(router, "symbol", http.HandlerFunc(pprof.Symbol))
	addPath(router, "trace", http.HandlerFunc(pprof.Trace))

	// The index handler only lists the runtime_pprof.Profiles() which does
	// not include all the endpoints we have. Thus we opt not to use the
	// common prof.Index endpoint here.
}
