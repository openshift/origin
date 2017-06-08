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

package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/broker/controller"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	"github.com/kubernetes-incubator/service-catalog/pkg/util"

	"github.com/gorilla/mux"
)

type server struct {
	controller controller.Controller
}

// CreateHandler creates Broker HTTP handler based on an implementation
// of a controller.Controller interface.
func CreateHandler(c controller.Controller) http.Handler {
	s := server{
		controller: c,
	}

	var router = mux.NewRouter()

	router.HandleFunc("/v2/catalog", s.catalog).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}", s.getServiceInstance).Methods("GET")
	router.HandleFunc("/v2/service_instances/{instance_id}", s.createServiceInstance).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}", s.removeServiceInstance).Methods("DELETE")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", s.bind).Methods("PUT")
	router.HandleFunc("/v2/service_instances/{instance_id}/service_bindings/{binding_id}", s.unBind).Methods("DELETE")

	return router
}

// Start creates the HTTP handler based on an implementation of a
// controller.Controller interface, and begins to listen on the specified port.
func Start(serverPort int, c controller.Controller) {
	glog.Infof("Starting server on %d\n", serverPort)
	http.Handle("/", CreateHandler(c))
	if err := http.ListenAndServe(":"+strconv.Itoa(serverPort), nil); err != nil {
		panic(err)
	}
}

func (s *server) catalog(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Get Service Broker Catalog...")

	if result, err := s.controller.Catalog(); err == nil {
		util.WriteResponse(w, http.StatusOK, result)
	} else {
		util.WriteResponse(w, http.StatusBadRequest, err)
	}
}

func (s *server) getServiceInstance(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["instance_id"]
	glog.Infof("GetServiceInstance ... %s\n", id)

	if result, err := s.controller.GetServiceInstance(id); err == nil {
		util.WriteResponse(w, http.StatusOK, result)
	} else {
		util.WriteResponse(w, http.StatusBadRequest, err)
	}
}

func (s *server) createServiceInstance(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["instance_id"]
	glog.Infof("CreateServiceInstance %s...\n", id)

	var req brokerapi.CreateServiceInstanceRequest
	if err := util.BodyToObject(r, &req); err != nil {
		glog.Errorf("error unmarshalling: %v", err)
		util.WriteResponse(w, http.StatusBadRequest, err)
		return
	}

	// TODO: Check if parameters are required, if not, this thing below is ok to leave in,
	// if they are ,they should be checked. Because if no parameters are passed in, this will
	// fail because req.Parameters is nil.
	if req.Parameters == nil {
		req.Parameters = make(map[string]interface{})
	}

	if result, err := s.controller.CreateServiceInstance(id, &req); err == nil {
		util.WriteResponse(w, http.StatusCreated, result)
	} else {
		util.WriteResponse(w, http.StatusBadRequest, err)
	}
}

func (s *server) removeServiceInstance(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["instance_id"]
	glog.Infof("RemoveServiceInstance %s...\n", id)

	if result, err := s.controller.RemoveServiceInstance(id); err == nil {
		util.WriteResponse(w, http.StatusOK, result)
	} else {
		util.WriteResponse(w, http.StatusBadRequest, err)
	}
}

func (s *server) bind(w http.ResponseWriter, r *http.Request) {
	bindingID := mux.Vars(r)["binding_id"]
	instanceID := mux.Vars(r)["instance_id"]

	glog.Infof("Bind binding_id=%s, instance_id=%s\n", bindingID, instanceID)

	var req brokerapi.BindingRequest

	if err := util.BodyToObject(r, &req); err != nil {
		glog.Errorf("Failed to unmarshall request: %v", err)
		util.WriteResponse(w, http.StatusBadRequest, err)
		return
	}

	// TODO: Check if parameters are required, if not, this thing below is ok to leave in,
	// if they are ,they should be checked. Because if no parameters are passed in, this will
	// fail because req.Parameters is nil.
	if req.Parameters == nil {
		req.Parameters = make(map[string]interface{})
	}

	// Pass in the instanceId to the template.
	req.Parameters["instanceId"] = instanceID

	if result, err := s.controller.Bind(instanceID, bindingID, &req); err == nil {
		util.WriteResponse(w, http.StatusOK, result)
	} else {
		util.WriteResponse(w, http.StatusBadRequest, err)
	}
}

func (s *server) unBind(w http.ResponseWriter, r *http.Request) {
	instanceID := mux.Vars(r)["instance_id"]
	bindingID := mux.Vars(r)["binding_id"]
	glog.Infof("UnBind: Service instance guid: %s:%s", bindingID, instanceID)

	if err := s.controller.UnBind(instanceID, bindingID); err == nil {
		w.WriteHeader(http.StatusOK)
		fmt.Print(w, "{}") //id)
	} else {
		util.WriteResponse(w, http.StatusBadRequest, err)
	}
}
