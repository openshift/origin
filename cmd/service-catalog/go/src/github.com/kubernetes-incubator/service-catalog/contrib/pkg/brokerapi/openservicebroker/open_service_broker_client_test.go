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

package openservicebroker

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/brokerapi"
	"github.com/kubernetes-incubator/service-catalog/contrib/pkg/brokerapi/openservicebroker/util"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

const (
	testClusterServiceBrokerName = "test-broker"
	bindingSuffixFormatString    = "/v2/service_instances/%s/service_bindings/%s"
	testServiceInstanceID        = "1"
	testServiceBindingID         = "2"
	testServiceID                = "3"
	testPlanID                   = "4"
	testOperation                = "testoperation"
)

func setup() (*util.FakeServiceBrokerServer, *servicecatalog.ClusterServiceBroker) {
	fbs := &util.FakeServiceBrokerServer{}
	url := fbs.Start()
	fakeClusterServiceBroker := &servicecatalog.ClusterServiceBroker{
		Spec: servicecatalog.ClusterServiceBrokerSpec{
			URL: url,
		},
	}

	return fbs, fakeClusterServiceBroker
}

func TestTrailingSlash(t *testing.T) {
	const (
		input    = "http://a/b/c/"
		expected = "http://a/b/c"
	)
	cl := NewClient("testClusterServiceBroker", input, "test-user", "test-pass")
	osbCl, ok := cl.(*openServiceBrokerClient)
	if !ok {
		t.Fatalf("NewClient didn't return an openServiceBrokerClient")
	}
	if osbCl.url != expected {
		t.Fatalf("URL was %s, expected %s", osbCl.url, expected)
	}
}

// Provision

func TestProvisionServiceInstanceCreated(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusCreated)
	_, rc, err := c.CreateServiceInstance(testServiceInstanceID, &brokerapi.CreateServiceInstanceRequest{})
	if err != nil {
		t.Fatal(err.Error())
	}

	if rc != http.StatusCreated {
		t.Fatalf("Expected '%d', got '%d'", http.StatusCreated, rc)
	}

	verifyRequestContentType(fbs.Request, t)
}

func TestProvisionServiceInstanceOK(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusOK)
	_, rc, err := c.CreateServiceInstance(testServiceInstanceID, &brokerapi.CreateServiceInstanceRequest{})
	if err != nil {
		t.Fatal(err.Error())
	}
	if rc != http.StatusOK {
		t.Fatalf("Expected '%d', got '%d'", http.StatusOK, rc)
	}
}

func TestProvisionServiceInstanceConflict(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusConflict)
	_, rc, err := c.CreateServiceInstance(testServiceInstanceID, &brokerapi.CreateServiceInstanceRequest{})
	if rc != http.StatusConflict {
		t.Fatalf("Expected '%d', got '%d'", http.StatusConflict, rc)
	}
	switch {
	case err == nil:
		t.Fatalf("Expected '%v'", errConflict)
	case err != errConflict:
		t.Fatalf("Expected '%v', got '%v'", errConflict, err)
	}
}

func TestProvisionServiceInstanceUnprocessableEntity(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusUnprocessableEntity)
	_, rc, err := c.CreateServiceInstance(testServiceInstanceID, &brokerapi.CreateServiceInstanceRequest{})
	if rc != http.StatusUnprocessableEntity {
		t.Fatalf("Expected '%d', got '%d'", http.StatusUnprocessableEntity, rc)
	}
	switch {
	case err == nil:
		t.Fatalf("Expected '%v'", errAsynchronous)
	case err != errAsynchronous:
		t.Fatalf("Expected '%v', got '%v'", errAsynchronous, err)
	}
}

func TestProvisionServiceInstanceAcceptedSuccessAsynchronous(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusAccepted)
	fbs.SetOperation(testOperation)
	req := brokerapi.CreateServiceInstanceRequest{
		AcceptsIncomplete: true,
	}

	resp, rc, err := c.CreateServiceInstance(testServiceInstanceID, &req)
	if err != nil {
		t.Fatal(err.Error())
	}
	if rc != http.StatusAccepted {
		t.Fatalf("Expected '%d', got '%d'", http.StatusAccepted, rc)
	}

	if resp.Operation != testOperation {
		t.Fatalf("Expected operation %q for async operation, got %q", testOperation, resp.Operation)
	}
}

// Deprovision

func TestDeprovisionServiceInstanceOK(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusOK)

	req := brokerapi.DeleteServiceInstanceRequest{
		ServiceID: testServiceID,
		PlanID:    testPlanID,
	}
	resp, rc, err := c.DeleteServiceInstance(testServiceInstanceID, &req)
	if err != nil {
		t.Fatal(err.Error())
	}
	if rc != http.StatusOK {
		t.Fatalf("Expected %d http status code, got %d", http.StatusOK, rc)
	}
	if resp.Operation != "" {
		t.Fatalf("Expected empty operation, got %q", resp.Operation)
	}

	expectedPath := fmt.Sprintf("/v2/service_instances/%s", testServiceInstanceID)
	verifyRequestMethodAndPath(http.MethodDelete, expectedPath, fbs.Request, t)
	verifyRequestParameter("service_id", testServiceID, fbs.Request, t)
	verifyRequestParameter("plan_id", testPlanID, fbs.Request, t)
}

func TestDeprovisionServiceInstanceGone(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusGone)
	resp, rc, err := c.DeleteServiceInstance(testServiceInstanceID, &brokerapi.DeleteServiceInstanceRequest{})
	if err != nil {
		t.Fatal(err.Error())
	}
	if rc != http.StatusGone {
		t.Fatalf("Expected %d http status code, got %d", http.StatusGone, rc)
	}
	if resp.Operation != "" {
		t.Fatalf("Expected empty operation, got %q", resp.Operation)
	}
}

func TestDeprovisionServiceInstanceUnprocessableEntity(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusUnprocessableEntity)
	resp, rc, err := c.DeleteServiceInstance(testServiceInstanceID, &brokerapi.DeleteServiceInstanceRequest{})
	if rc != http.StatusUnprocessableEntity {
		t.Fatalf("Expected %d http status code, got %d", http.StatusUnprocessableEntity, rc)
	}
	if resp.Operation != "" {
		t.Fatalf("Expected empty operation, got %q", resp.Operation)
	}
	switch {
	case err == nil:
		t.Fatalf("Expected '%v'", errAsynchronous)
	case err != errAsynchronous:
		t.Fatalf("Expected '%v', got '%v'", errAsynchronous, err)
	}
}

func TestDeprovisionServiceInstanceAcceptedSuccessAsynchronous(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusAccepted)
	fbs.SetOperation(testOperation)
	req := brokerapi.DeleteServiceInstanceRequest{
		AcceptsIncomplete: true,
	}

	resp, rc, err := c.DeleteServiceInstance(testServiceInstanceID, &req)
	if err != nil {
		t.Fatal(err.Error())
	}
	if rc != http.StatusAccepted {
		t.Fatalf("Expected %d http status code, got %d", http.StatusAccepted, rc)
	}
	if resp.Operation != testOperation {
		t.Fatalf("Expected operation %q for async operation, got %q", testOperation, resp.Operation)
	}

}

func TestBindOk(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusOK)
	sent := &brokerapi.BindingRequest{}
	if _, err := c.CreateServiceBinding(testServiceInstanceID, testServiceBindingID, sent); err != nil {
		t.Fatal(err.Error())
	}

	verifyServiceBindingMethodAndPath(http.MethodPut, testServiceInstanceID, testServiceBindingID, fbs.Request, t)

	if fbs.RequestObject == nil {
		t.Fatalf("ServiceBindingRequest was not received correctly")
	}
	verifyRequestContentType(fbs.Request, t)

	actual := reflect.TypeOf(fbs.RequestObject)
	expected := reflect.TypeOf(&brokerapi.BindingRequest{})
	if actual != expected {
		t.Fatalf("Got the wrong type for the request, expected %v got %v", expected, actual)
	}
	received := fbs.RequestObject.(*brokerapi.BindingRequest)
	if !reflect.DeepEqual(*received, *sent) {
		t.Fatalf("Sent does not match received, sent: %+v received: %+v", sent, received)
	}
}

func TestBindConflict(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusConflict)
	sent := &brokerapi.BindingRequest{}
	if _, err := c.CreateServiceBinding(testServiceInstanceID, testServiceBindingID, sent); err == nil {
		t.Fatal("Expected create service binding to fail with conflict, but didn't")
	}

	verifyServiceBindingMethodAndPath(http.MethodPut, testServiceInstanceID, testServiceBindingID, fbs.Request, t)

	if fbs.RequestObject == nil {
		t.Fatalf("ServiceBindingRequest was not received correctly")
	}
	actual := reflect.TypeOf(fbs.RequestObject)
	expected := reflect.TypeOf(&brokerapi.BindingRequest{})
	if actual != expected {
		t.Fatalf("Got the wrong type for the request, expected %v got %v", expected, actual)
	}
	received := fbs.RequestObject.(*brokerapi.BindingRequest)
	if !reflect.DeepEqual(*received, *sent) {
		t.Fatalf("Sent does not match received, sent: %+v received: %+v", sent, received)
	}
}

func TestUnbindOk(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusOK)
	if err := c.DeleteServiceBinding(testServiceInstanceID, testServiceBindingID, testServiceID, testPlanID); err != nil {
		t.Fatal(err.Error())
	}

	verifyServiceBindingMethodAndPath(http.MethodDelete, testServiceInstanceID, testServiceBindingID, fbs.Request, t)
	verifyRequestParameter("service_id", testServiceID, fbs.Request, t)
	verifyRequestParameter("plan_id", testPlanID, fbs.Request, t)

	if fbs.Request.ContentLength != 0 {
		t.Fatalf("not expecting a request body, but got one, size %d", fbs.Request.ContentLength)
	}
}

func TestUnbindGone(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")

	fbs.SetResponseStatus(http.StatusGone)
	err := c.DeleteServiceBinding(testServiceInstanceID, testServiceBindingID, testServiceID, testPlanID)
	if err == nil {
		t.Fatal("Expected delete service binding to fail with gone, but didn't")
	}
	if !strings.Contains(err.Error(), "There is no binding") {
		t.Fatalf("Did not find the expected error message 'There is no binding' in error: %s", err)
	}

	verifyServiceBindingMethodAndPath(http.MethodDelete, testServiceInstanceID, testServiceBindingID, fbs.Request, t)
}

func TestPollServiceInstanceWithMissingServiceID(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")
	r := &brokerapi.LastOperationRequest{PlanID: testPlanID}
	_, _, err := c.PollServiceInstance(testServiceInstanceID, r)
	if err == nil {
		t.Fatal("PollServiceInstance did not fail with invalid LastOperationRequest")
	}
	if !strings.Contains(err.Error(), "missing service_id") {
		t.Fatalf("Did not find the expected error message 'missing service_id' in error: %s", err)
	}
}

func TestPollServiceInstanceWithMissingPlanID(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")
	r := &brokerapi.LastOperationRequest{ServiceID: testServiceID}
	_, _, err := c.PollServiceInstance(testServiceInstanceID, r)
	if err == nil {
		t.Fatal("PollServiceInstance did not fail with invalid LastOperationRequest")
	}
	if !strings.Contains(err.Error(), "missing plan_id") {
		t.Fatalf("Did not find the expected error message 'missing plan_id' in error: %s", err)
	}
}

func TestPollServiceInstanceWithFailure(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")
	fbs.SetResponseStatus(http.StatusBadRequest)
	r := &brokerapi.LastOperationRequest{ServiceID: testServiceID, PlanID: testPlanID, Operation: testOperation}
	_, rc, err := c.PollServiceInstance(testServiceInstanceID, r)
	if err == nil {
		t.Fatal("PollServiceInstance did not fail with statusBadRequest")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("Did not find the expected error message '400' error: %s", err)
	}
	if rc != http.StatusBadRequest {
		t.Fatalf("Expected http status %d but got %d", http.StatusOK, rc)
	}
}

func TestPollServiceInstanceWithGone(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")
	fbs.SetResponseStatus(http.StatusGone)
	r := &brokerapi.LastOperationRequest{ServiceID: testServiceID, PlanID: testPlanID, Operation: testOperation}
	_, rc, err := c.PollServiceInstance(testServiceInstanceID, r)
	if err == nil {
		t.Fatal("PollServiceInstance did not fail with statusBadRequest")
	}
	if rc != http.StatusGone {
		t.Fatalf("Expected http status %d but got %d", http.StatusOK, rc)
	}
}

func TestPollServiceInstanceWithSuccess(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")
	fbs.SetResponseStatus(http.StatusOK)
	fbs.SetLastOperationState("success")
	r := &brokerapi.LastOperationRequest{ServiceID: testServiceID, PlanID: testPlanID, Operation: testOperation}
	resp, rc, err := c.PollServiceInstance(testServiceInstanceID, r)
	if err != nil {
		t.Fatalf("PollServiceInstance failed unexpectedly with: %s", err)
	}

	expectedPath := fmt.Sprintf("/v2/service_instances/%s/last_operation", testServiceInstanceID)
	verifyRequestMethodAndPath(http.MethodGet, expectedPath, fbs.Request, t)
	verifyRequestParameter("service_id", testServiceID, fbs.Request, t)
	verifyRequestParameter("plan_id", testPlanID, fbs.Request, t)
	verifyRequestParameter("operation", testOperation, fbs.Request, t)
	if rc != http.StatusOK {
		t.Fatalf("Expected http status %d but got %d", http.StatusOK, rc)
	}
	if resp.State != "success" {
		t.Fatalf("Did not receive state %q for last_operation_request, got: %q", "success", resp.State)
	}
	if resp.Description == "" {
		t.Fatalf("Did not receive description for last_operation_request, got: %+v", resp)
	}
}

func TestPollServiceInstanceWithNoOperation(t *testing.T) {
	fbs, fakeClusterServiceBroker := setup()
	defer fbs.Stop()

	c := NewClient(testClusterServiceBrokerName, fakeClusterServiceBroker.Spec.URL, "", "")
	fbs.SetResponseStatus(http.StatusOK)
	fbs.SetLastOperationState("failed")
	r := &brokerapi.LastOperationRequest{ServiceID: testServiceID, PlanID: testPlanID}
	resp, rc, err := c.PollServiceInstance(testServiceInstanceID, r)
	if err != nil {
		t.Fatalf("PollServiceInstance failed unexpectedly with: %s", err)
	}

	expectedPath := fmt.Sprintf("/v2/service_instances/%s/last_operation", testServiceInstanceID)
	verifyRequestMethodAndPath(http.MethodGet, expectedPath, fbs.Request, t)
	verifyRequestParameter("service_id", testServiceID, fbs.Request, t)
	verifyRequestParameter("plan_id", testPlanID, fbs.Request, t)
	// Make sure operation is not set.
	verifyRequestParameter("operation", "", fbs.Request, t)
	if rc != http.StatusOK {
		t.Fatalf("Expected http status %d but got %d", http.StatusOK, rc)
	}
	if resp.State != "failed" {
		t.Fatalf("Did not receive state %q for last_operation_request, got: %q", "success", resp.State)
	}
	if resp.Description == "" {
		t.Fatalf("Did not receive description for last_operation_request, got: %+v", resp)
	}
}

// verifyServiceBindingMethodAndPath is a helper method that verifies that the request
// has the right method and the suffix URL for a binding request.
func verifyServiceBindingMethodAndPath(method, serviceID, bindingID string, req *http.Request, t *testing.T) {
	expectedPath := fmt.Sprintf(bindingSuffixFormatString, serviceID, bindingID)
	verifyRequestMethodAndPath(method, expectedPath, req, t)
}

func verifyRequestMethodAndPath(method, expectedPath string, req *http.Request, t *testing.T) {
	if req.Method != method {
		t.Fatalf("Expected method to use %s but was %s", method, req.Method)
	}
	if !strings.HasSuffix(req.URL.Path, expectedPath) {
		t.Fatalf("Expected request path to end with %s but was: %s", expectedPath, req.URL.Path)
	}
}

func verifyRequestParameter(paramName string, expectedValue string, req *http.Request, t *testing.T) {
	actualValue := req.FormValue(paramName)
	if actualValue != expectedValue {
		t.Fatalf("Expected %s parameter to be %s, but was %s", paramName, expectedValue, actualValue)
	}
}

func verifyRequestContentType(req *http.Request, t *testing.T) {
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Fatalf("Expected the request content-type to be application/json, but was %s", contentType)
	}
}
