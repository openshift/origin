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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/broker/controller"
	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/brokerapi"
)

//
// Test of server /v2/catalog endpoint.
//

// Make sure that Controller stub implements controller.Controller interface
var _ controller.Controller = &Controller{}

// /v2/catalog returns HTTP error on error.
func TestCatalogReturnsHTTPErrorOnError(t *testing.T) {
	handler := createHandler(&Controller{
		t: t,
		catalog: func() (*brokerapi.Catalog, error) {
			return nil, errors.New("Catalog retrieval error")
		},
	})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/v2/catalog", nil))

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected HTTP status http.StatusBadRequest (%d), got %d", http.StatusBadRequest, rr.Code)
	}

	if contentType := rr.Header().Get("content-type"); contentType != "application/json" {
		t.Errorf("Expected response content-type 'application/json', got '%s'", contentType)
	}

	if body := rr.Body.String(); body != `{"Error":"Catalog retrieval error"}` {
		t.Errorf("Expected structured error response; got '%s'", body)
	}
}

// /v2/catalog returns compliant JSON
func TestCatalogReturnsCompliantJSON(t *testing.T) {
	handler := createHandler(&Controller{
		t: t,
		catalog: func() (*brokerapi.Catalog, error) {
			return &brokerapi.Catalog{Services: []*brokerapi.Service{
				{
					Name: "foo",
				},
			}}, nil
		}})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest("GET", "/v2/catalog", nil))

	if rr.Code != http.StatusOK {
		t.Errorf("Expected HTTP status http.StatusOK (%d), got %d", http.StatusOK, rr.Code)
	}

	if contentType := rr.Header().Get("content-type"); contentType != "application/json" {
		t.Errorf("Expected response content-type 'application/json', got '%s'", contentType)
	}

	catalog, err := readJSON(rr)
	if err != nil {
		t.Errorf("Failed to parse JSON response with error %v", err)
	}

	if len(catalog) != 1 {
		t.Errorf("Expected catalog to have 1 element, got %d", len(catalog))
	}

	if _, ok := catalog["services"]; !ok {
		t.Errorf("Expected catalog %v to contain key 'services'", catalog)
	}

	services := catalog["services"].([]interface{})
	if services == nil {
		t.Error("Expected 'services' property of the returned catalog to be not nil, got nil")
	}

	var service map[string]interface{}
	service = services[0].(map[string]interface{})

	if name, ok := service["name"]; !ok {
		t.Error("Returned service doesn't have a 'name' property.")
	} else if name != "foo" {
		t.Errorf("Expected returned service name to be 'foo', got '%s'", name)
	}
}

func readJSON(rr *httptest.ResponseRecorder) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &result)
	return result, err
}

type Controller struct {
	t *testing.T

	catalog                         func() (*brokerapi.Catalog, error)
	getServiceInstanceLastOperation func(id string) (*brokerapi.LastOperationResponse, error)
	createServiceInstance           func(id string, req *brokerapi.CreateServiceInstanceRequest) (*brokerapi.CreateServiceInstanceResponse, error)
	removeServiceInstance           func(id string) (*brokerapi.DeleteServiceInstanceResponse, error)
	bind                            func(instanceID string, bindingID string, req *brokerapi.BindingRequest) (*brokerapi.CreateServiceBindingResponse, error)
	unBind                          func(instanceID string, bindingID string) error
}

func (controller *Controller) Catalog() (*brokerapi.Catalog, error) {
	if controller.catalog == nil {
		controller.t.Error("Test failed to provide 'catalog' handler")
	}

	return controller.catalog()
}

func (controller *Controller) GetServiceInstanceLastOperation(instanceID, serviceID, planID, operation string) (*brokerapi.LastOperationResponse, error) {
	if controller.getServiceInstanceLastOperation == nil {
		controller.t.Error("Test failed to provide 'getServiceInstanceLastOperation' handler")
	}

	return controller.getServiceInstanceLastOperation(instanceID)
}

func (controller *Controller) CreateServiceInstance(id string, req *brokerapi.CreateServiceInstanceRequest) (*brokerapi.CreateServiceInstanceResponse, error) {
	if controller.createServiceInstance == nil {
		controller.t.Error("Test failed to provide 'createServiceInstance' handler")
	}

	return controller.createServiceInstance(id, req)
}

func (controller *Controller) RemoveServiceInstance(instanceID, serviceID, planID string, acceptsIncomplete bool) (*brokerapi.DeleteServiceInstanceResponse, error) {
	if controller.removeServiceInstance == nil {
		controller.t.Error("Test failed to provide 'removeServiceInstance' handler")
	}

	return controller.removeServiceInstance(instanceID)
}

func (controller *Controller) Bind(instanceID string, bindingID string, req *brokerapi.BindingRequest) (*brokerapi.CreateServiceBindingResponse, error) {
	if controller.bind == nil {
		controller.t.Error("Test failed to provide 'bind' handler")
	}

	return controller.bind(instanceID, bindingID, req)
}

func (controller *Controller) UnBind(instanceID, bindingID, serviceID, planID string) error {
	if controller.unBind == nil {
		controller.t.Error("Test failed to provide 'unBind' handler")
	}

	return controller.unBind(instanceID, bindingID)
}
