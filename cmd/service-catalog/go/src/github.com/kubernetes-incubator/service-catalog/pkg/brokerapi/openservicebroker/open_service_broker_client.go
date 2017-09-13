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
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	"github.com/kubernetes-incubator/service-catalog/pkg/util"

	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/openservicebroker/constants"
)

const (
	catalogFormatString         = "%s/v2/catalog"
	serviceInstanceFormatString = "%s/v2/service_instances/%s"
	pollingFormatString         = "%s/v2/service_instances/%s/last_operation"
	bindingFormatString         = "%s/v2/service_instances/%s/service_bindings/%s"

	httpTimeoutSeconds     = 15
	pollingIntervalSeconds = 1
	pollingAmountLimit     = 30
)

var (
	errConflict        = errors.New("Service instance with same id but different attributes exists")
	errBindingConflict = errors.New("Service binding with same service instance id and binding id already exists")
	errBindingGone     = errors.New("There is no binding with the specified service instance id and binding id")
	errAsynchronous    = errors.New("ServiceBroker only supports this action asynchronously")
	errFailedState     = errors.New("Failed state received from broker")
	errUnknownState    = errors.New("Unknown state received from broker")
	errPollingTimeout  = errors.New("Timed out while polling broker")
)

type (
	errRequest struct {
		message string
	}

	errResponse struct {
		message string
	}

	errStatusCode struct {
		statusCode int
	}
)

func (e errRequest) Error() string {
	return fmt.Sprintf("Failed to send request: %s", e.message)
}

func (e errResponse) Error() string {
	return fmt.Sprintf("Failed to parse broker response: %s", e.message)
}

func (e errStatusCode) Error() string {
	return fmt.Sprintf("Unexpected status code from broker response: %v", e.statusCode)
}

type openServiceBrokerClient struct {
	name     string
	url      string
	username string
	password string
	*http.Client
}

// NewClient creates an instance of BrokerClient for communicating with brokers
// which implement the Open Service Broker API.
func NewClient(name, url, username, password string) brokerapi.BrokerClient {
	// TODO(vaikas): Make this into a flag/config option. Necessary to talk to brokers that
	// have non-root signed certs.
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	return &openServiceBrokerClient{
		name:     name,
		url:      strings.TrimRight(url, "/"), // remove trailing slashes from broker server URLs
		username: username,
		password: password,
		Client: &http.Client{
			Timeout:   httpTimeoutSeconds * time.Second,
			Transport: tr,
		},
	}
}

func (c *openServiceBrokerClient) GetCatalog() (*brokerapi.Catalog, error) {
	catalogURL := fmt.Sprintf(catalogFormatString, c.url)

	req, err := c.newOSBRequest(http.MethodGet, catalogURL, nil, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(c.username, c.password)
	resp, err := c.Do(req)
	if err != nil {
		glog.Errorf("Failed to fetch catalog %q from %s: response: %v error: %#v", c.name, catalogURL, resp, err)
		return nil, err
	}

	var catalog brokerapi.Catalog
	if err = util.ResponseBodyToObject(resp, &catalog); err != nil {
		glog.Errorf("Failed to unmarshal catalog from broker %q: %#v", c.name, err)
		return nil, err
	}

	return &catalog, nil
}

func (c *openServiceBrokerClient) CreateServiceInstance(ID string, req *brokerapi.CreateServiceInstanceRequest) (*brokerapi.CreateServiceInstanceResponse, int, error) {
	serviceInstanceURL := fmt.Sprintf(serviceInstanceFormatString, c.url, ID)

	resp, err := sendOSBRequest(
		c,
		http.MethodPut,
		serviceInstanceURL,
		map[string]string{
			"accepts_incomplete": fmt.Sprintf("%t", req.AcceptsIncomplete),
		},
		req,
	)
	if err != nil {
		glog.Errorf("Error sending create service instance request to broker %q at %v: response: %v error: %#v", c.name, serviceInstanceURL, resp, err)
		errReq := errRequest{message: err.Error()}
		if resp == nil {
			return nil, 0, errReq
		}
		return nil, resp.StatusCode, errReq
	}
	defer resp.Body.Close()

	createServiceInstanceResponse := brokerapi.CreateServiceInstanceResponse{}
	if err := util.ResponseBodyToObject(resp, &createServiceInstanceResponse); err != nil {
		glog.Errorf("Error unmarshalling create service instance response from broker %q: %#v", c.name, err)
		return nil, resp.StatusCode, errResponse{message: err.Error()}
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return &createServiceInstanceResponse, resp.StatusCode, nil
	case http.StatusOK:
		return &createServiceInstanceResponse, resp.StatusCode, nil
	case http.StatusAccepted:
		glog.V(3).Infof("Asynchronous response received.")
		return &createServiceInstanceResponse, resp.StatusCode, nil
	case http.StatusConflict:
		return nil, resp.StatusCode, errConflict
	case http.StatusUnprocessableEntity:
		return nil, resp.StatusCode, errAsynchronous
	default:
		return nil, resp.StatusCode, errStatusCode{statusCode: resp.StatusCode}
	}
}

func (c *openServiceBrokerClient) UpdateServiceInstance(ID string, req *brokerapi.CreateServiceInstanceRequest) (*brokerapi.ServiceInstance, int, error) {
	// TODO: https://github.com/kubernetes-incubator/service-catalog/issues/114
	return nil, 0, fmt.Errorf("Not implemented")
}

func (c *openServiceBrokerClient) DeleteServiceInstance(ID string, req *brokerapi.DeleteServiceInstanceRequest) (*brokerapi.DeleteServiceInstanceResponse, int, error) {
	serviceInstanceURL := fmt.Sprintf(serviceInstanceFormatString, c.url, ID)

	resp, err := sendOSBRequest(
		c,
		http.MethodDelete,
		serviceInstanceURL,
		map[string]string{
			"service_id":         req.ServiceID,
			"plan_id":            req.PlanID,
			"accepts_incomplete": fmt.Sprintf("%t", req.AcceptsIncomplete),
		},
		req,
	)
	if err != nil {
		glog.Errorf("Error sending delete service instance request to broker %q at %v: response: %v error: %#v", c.name, serviceInstanceURL, resp, err)
		return nil, resp.StatusCode, errRequest{message: err.Error()}
	}
	defer resp.Body.Close()

	deleteServiceInstanceResponse := brokerapi.DeleteServiceInstanceResponse{}
	if err := util.ResponseBodyToObject(resp, &deleteServiceInstanceResponse); err != nil {
		glog.Errorf("Error unmarshalling delete service instance response from broker %q: %#v", c.name, err)
		return nil, resp.StatusCode, errResponse{message: err.Error()}
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return &deleteServiceInstanceResponse, resp.StatusCode, nil
	case http.StatusAccepted:
		glog.V(3).Infof("Asynchronous response received.")
		return &deleteServiceInstanceResponse, resp.StatusCode, nil
	case http.StatusGone:
		return &deleteServiceInstanceResponse, resp.StatusCode, nil
	case http.StatusUnprocessableEntity:
		return &deleteServiceInstanceResponse, resp.StatusCode, errAsynchronous
	default:
		return &deleteServiceInstanceResponse, resp.StatusCode, errStatusCode{statusCode: resp.StatusCode}
	}
}

func (c *openServiceBrokerClient) CreateServiceBinding(instanceID, bindingID string, req *brokerapi.BindingRequest) (*brokerapi.CreateServiceBindingResponse, error) {
	jsonBytes, err := json.Marshal(req)
	if err != nil {
		glog.Errorf("Failed to marshal: %#v", err)
		return nil, err
	}

	serviceBindingURL := fmt.Sprintf(bindingFormatString, c.url, instanceID, bindingID)

	createHTTPReq, err := c.newOSBRequest(
		http.MethodPut,
		serviceBindingURL,
		nil,
		bytes.NewReader(jsonBytes),
	)
	if err != nil {
		return nil, err
	}

	glog.Infof("Doing a request to: %s", serviceBindingURL)
	resp, err := c.Do(createHTTPReq)
	if err != nil {
		glog.Errorf("Failed to PUT: %#v", err)
		return nil, err
	}
	defer resp.Body.Close()

	createServiceBindingResponse := brokerapi.CreateServiceBindingResponse{}
	if err := util.ResponseBodyToObject(resp, &createServiceBindingResponse); err != nil {
		glog.Errorf("Error unmarshalling create binding response from broker: %#v", err)
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusCreated:
		return &createServiceBindingResponse, nil
	case http.StatusOK:
		return &createServiceBindingResponse, nil
	case http.StatusConflict:
		return nil, errBindingConflict
	default:
		return nil, errStatusCode{statusCode: resp.StatusCode}
	}
}

func (c *openServiceBrokerClient) DeleteServiceBinding(instanceID, bindingID, serviceID, planID string) error {
	serviceBindingURL := fmt.Sprintf(bindingFormatString, c.url, instanceID, bindingID)

	deleteHTTPReq, err := c.newOSBRequest(
		http.MethodDelete,
		serviceBindingURL,
		map[string]string{
			"service_id": serviceID,
			"plan_id":    planID,
		},
		nil,
	)
	if err != nil {
		glog.Errorf("Failed to create new HTTP request: %v", err)
		return err
	}

	glog.Infof("Doing a request to: %s", serviceBindingURL)
	resp, err := c.Do(deleteHTTPReq)
	if err != nil {
		glog.Errorf("Failed to DELETE: %#v", err)
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusGone:
		return errBindingGone
	default:
		return errStatusCode{statusCode: resp.StatusCode}
	}

}

func (c *openServiceBrokerClient) PollServiceInstance(ID string, req *brokerapi.LastOperationRequest) (*brokerapi.LastOperationResponse, int, error) {
	if req.ServiceID == "" {
		return nil, 0, fmt.Errorf("LastOperationRequest is missing service_id")
	}
	if req.PlanID == "" {
		return nil, 0, fmt.Errorf("LastOperationRequest is missing plan_id")
	}
	url := fmt.Sprintf(pollingFormatString, c.url, ID)
	resp, err := sendOSBRequest(
		c,
		http.MethodGet,
		url,
		map[string]string{
			"service_id": req.ServiceID,
			"plan_id":    req.PlanID,
			"operation":  req.Operation,
		},
		nil,
	)
	if err != nil {
		glog.Errorf("Failed to create new HTTP request: %v", err)
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, errStatusCode{statusCode: resp.StatusCode}
	}

	lo := brokerapi.LastOperationResponse{}
	if err := util.ResponseBodyToObject(resp, &lo); err != nil {
		return nil, resp.StatusCode, err
	}
	return &lo, resp.StatusCode, nil
}

// SendRequest will serialize 'object' and send it using the given method to
// the given URL, through the provided client
func sendOSBRequest(
	c *openServiceBrokerClient,
	method string,
	url string,
	queryParams map[string]string,
	object interface{},
) (*http.Response, error) {
	data, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal request: %s", err.Error())
	}

	req, err := c.newOSBRequest(method, url, queryParams, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("Failed to create request object: %s", err.Error())
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Failed to send request: %s", err.Error())
	}

	return resp, nil
}

func (c *openServiceBrokerClient) newOSBRequest(
	method string,
	urlStr string,
	queryParams map[string]string,
	body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Add(constants.APIVersionHeader, constants.APIVersion)
	req.SetBasicAuth(c.username, c.password)
	if queryParams != nil {
		q := req.URL.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	return req, nil
}
