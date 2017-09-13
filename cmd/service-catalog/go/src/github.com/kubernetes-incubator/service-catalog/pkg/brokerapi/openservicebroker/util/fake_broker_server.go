/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"net/http"
	"net/http/httptest"

	"github.com/golang/glog"
	"github.com/gorilla/mux"

	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/openservicebroker/constants"
	"github.com/kubernetes-incubator/service-catalog/pkg/util"
)

const asyncProvisionQueryParamKey = "accepts_incomplete"

// LastOperationResponseTestDescription is returned as the description to
// last_operation requests.
const LastOperationResponseTestDescription = "test description for last operation"

// FakeServiceBrokerServer is a fake service broker server meant for testing that
// allows for customizing the response behavior.  It does not support auth.
type FakeServiceBrokerServer struct {
	responseStatus     int
	operation          string
	lastOperationState string
	server             *httptest.Server
	// For inspecting on what was sent on the wire.
	RequestObject interface{}
	Request       *http.Request
}

// Start starts the fake broker server listening on a random port, passing
// back the server's URL.
func (f *FakeServiceBrokerServer) Start() string {
	r := mux.NewRouter()
	// check for headers required by osb api spec
	router := r.Headers(constants.APIVersionHeader, "",
		"Authorization", "",
	).Subrouter()

	router.HandleFunc("/v2/catalog", f.catalogHandler).Methods("GET")
	router.HandleFunc("/v2/service_instances/{id}/last_operation", f.lastOperationHandler).Methods("GET")
	router.HandleFunc("/v2/service_instances/{id}", f.provisionHandler).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{id}", f.updateHandler).Methods("PATCH")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", f.bindHandler).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", f.unbindHandler).Methods("DELETE")
	router.HandleFunc("/v2/service_instances/{id}", f.deprovisionHandler).Methods("DELETE")

	f.server = httptest.NewServer(r)
	return f.server.URL
}

// Stop shuts down the server.
func (f *FakeServiceBrokerServer) Stop() {
	f.server.Close()
	glog.Info("fake broker stopped")
}

// SetResponseStatus sets the default response status of the broker to the
// given HTTP status code.
func (f *FakeServiceBrokerServer) SetResponseStatus(status int) {
	f.responseStatus = status
}

// SetOperation sets the operation to return for asynchronous operations.
func (f *FakeServiceBrokerServer) SetOperation(operation string) {
	f.operation = operation
}

// SetLastOperationState sets the state to return for last_operation requests.
func (f *FakeServiceBrokerServer) SetLastOperationState(state string) {
	f.lastOperationState = state
}

// HANDLERS

func (f *FakeServiceBrokerServer) catalogHandler(w http.ResponseWriter, r *http.Request) {
	glog.Info("fake catalog called")
	util.WriteResponse(w, http.StatusOK, &brokerapi.Catalog{})
}

func (f *FakeServiceBrokerServer) lastOperationHandler(w http.ResponseWriter, r *http.Request) {
	glog.Info("fake lastOperation called")
	f.Request = r
	req := &brokerapi.LastOperationRequest{}
	if err := util.BodyToObject(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.RequestObject = req

	state := "in progress"
	if f.lastOperationState != "" {
		state = f.lastOperationState
	}

	resp := brokerapi.LastOperationResponse{
		State:       state,
		Description: LastOperationResponseTestDescription,
	}
	util.WriteResponse(w, f.responseStatus, &resp)
}

func (f *FakeServiceBrokerServer) provisionHandler(w http.ResponseWriter, r *http.Request) {
	glog.Info("fake provision called")
	f.Request = r
	req := &brokerapi.CreateServiceInstanceRequest{}
	if err := util.BodyToObject(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.RequestObject = req

	if r.FormValue(asyncProvisionQueryParamKey) != "true" {
		// Synchronous
		util.WriteResponse(w, f.responseStatus, &brokerapi.CreateServiceInstanceResponse{})
	} else {
		// Asynchronous
		resp := brokerapi.CreateServiceInstanceResponse{
			Operation: f.operation,
		}
		util.WriteResponse(w, http.StatusAccepted, &resp)
	}
}

func (f *FakeServiceBrokerServer) deprovisionHandler(w http.ResponseWriter, r *http.Request) {
	glog.Info("fake deprovision called")
	f.Request = r
	req := &brokerapi.DeleteServiceInstanceRequest{
		ServiceID: r.URL.Query().Get("service_id"),
		PlanID:    r.URL.Query().Get("plan_id"),
	}
	incompleteStr := r.URL.Query().Get("accepts_incomplete")
	if incompleteStr == "true" {
		req.AcceptsIncomplete = true
	}

	f.RequestObject = req

	if r.FormValue(asyncProvisionQueryParamKey) != "true" {
		// Synchronous
		util.WriteResponse(w, f.responseStatus, &brokerapi.DeleteServiceInstanceResponse{})
	} else {
		// Asynchronous
		resp := brokerapi.CreateServiceInstanceResponse{
			Operation: f.operation,
		}
		util.WriteResponse(w, http.StatusAccepted, &resp)
	}
}

func (f *FakeServiceBrokerServer) updateHandler(w http.ResponseWriter, r *http.Request) {
	glog.Info("fake update called")
	// TODO: Implement
	util.WriteResponse(w, http.StatusForbidden, nil)
}

func (f *FakeServiceBrokerServer) bindHandler(w http.ResponseWriter, r *http.Request) {
	glog.Info("fake bind called")
	f.Request = r
	req := &brokerapi.BindingRequest{}
	if err := util.BodyToObject(r, req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.RequestObject = req
	util.WriteResponse(w, f.responseStatus, &brokerapi.DeleteServiceInstanceResponse{})
}

func (f *FakeServiceBrokerServer) unbindHandler(w http.ResponseWriter, r *http.Request) {
	glog.Info("fake unbind called")
	f.Request = r
	util.WriteResponse(w, f.responseStatus, &brokerapi.DeleteServiceInstanceResponse{})
}
