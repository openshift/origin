//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package admin

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/heketi/heketi/pkg/glusterfs/api"
	"github.com/heketi/heketi/pkg/utils"
)

var (
	// there's got to be a better way than this, but I couldn't find
	// one in a reasonable number of searches
	localhost4 = "127.0.0.1"
	localhost6 = "::1"
)

func (s *ServerState) AllowRequest(r *http.Request) bool {
	switch s.Get() {
	case api.AdminStateReadOnly:
		return r.Method == http.MethodGet || r.Method == http.MethodHead
	case api.AdminStateLocal:
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			return true
		}
		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		return clientIP == localhost4 || clientIP == localhost6
	default:
		return true
	}
}

func (s *ServerState) ServeHTTP(
	w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {

	if !s.AllowRequest(r) {
		http.Error(w,
			"Service disabled for maintenance",
			http.StatusServiceUnavailable)
		return
	}
	next(w, r)
}

func (s *ServerState) SetRoutes(router *mux.Router) error {
	router.
		Methods("GET").
		Path("/admin").
		Name("GetAdminState").
		Handler(http.HandlerFunc(s.GetAdminState))
	router.
		Methods("POST").
		Path("/admin").
		Name("SetAdminState").
		Handler(http.HandlerFunc(s.SetAdminState))
	return nil
}

func (s *ServerState) GetAdminState(w http.ResponseWriter, r *http.Request) {
	data := api.AdminStatus{
		State: s.Get(),
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		panic(err)
	}
}

func (s *ServerState) SetAdminState(w http.ResponseWriter, r *http.Request) {
	msg := api.AdminStatus{}
	err := utils.GetJsonFromRequest(r, &msg)
	if err != nil {
		http.Error(w,
			fmt.Sprintf("request unable to be parsed: %s", err.Error()),
			http.StatusBadRequest)
		return
	}

	if err := msg.Validate(); err != nil {
		http.Error(w, "validation failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.Set(msg.State)
	w.WriteHeader(http.StatusNoContent)
}
