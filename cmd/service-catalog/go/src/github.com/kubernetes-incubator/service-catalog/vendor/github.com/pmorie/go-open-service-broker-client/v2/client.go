package v2

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/golang/glog"
)

const (
	// XBrokerAPIVersion is the header for the Open Service Broker API
	// version.
	XBrokerAPIVersion = "X-Broker-API-Version"

	catalogURL            = "%s/v2/catalog"
	serviceInstanceURLFmt = "%s/v2/service_instances/%s"
	lastOperationURLFmt   = "%s/v2/service_instances/%s/last_operation"
	bindingURLFmt         = "%s/v2/service_instances/%s/service_bindings/%s"
	asyncQueryParamKey    = "accepts_incomplete"
)

// NewClient is a CreateFunc for creating a new functional Client and
// implements the CreateFunc interface.
func NewClient(config *ClientConfiguration) (Client, error) {
	httpClient := &http.Client{
		Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
	}
	transport := &http.Transport{}
	if config.TLSConfig != nil {
		transport.TLSClientConfig = config.TLSConfig
	} else {
		transport.TLSClientConfig = &tls.Config{}
	}
	if config.Insecure {
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	httpClient.Transport = transport

	c := &client{
		Name:                config.Name,
		URL:                 strings.TrimRight(config.URL, "/"),
		APIVersion:          config.APIVersion,
		EnableAlphaFeatures: config.EnableAlphaFeatures,
		httpClient:          httpClient,
	}
	c.doRequestFunc = c.doRequest

	if config.AuthConfig != nil {
		if config.AuthConfig.BasicAuthConfig == nil {
			return nil, errors.New("BasicAuthConfig is required if AuthConfig is provided")
		}

		c.BasicAuthConfig = config.AuthConfig.BasicAuthConfig
	}

	return c, nil
}

var _ CreateFunc = NewClient

type doRequestFunc func(request *http.Request) (*http.Response, error)

// client provides a functional implementation of the Client interface.
type client struct {
	Name                string
	URL                 string
	APIVersion          APIVersion
	BasicAuthConfig     *BasicAuthConfig
	EnableAlphaFeatures bool
	Verbose             bool

	httpClient    *http.Client
	doRequestFunc doRequestFunc
}

var _ Client = &client{}

// This file contains shared methods used by each interface method of the
// Client interface.  Individual interface methods are in the following files:
//
// GetCatalog: get_catalog.go
// ProvisionInstance: provision_instance.go
// UpdateInstance: update_instance.go
// DeprovisionInstance: deprovision_instance.go
// PollLastOperation: poll_last_operation.go
// Bind: bind.go
// Unbind: unbind.go

const (
	contentType = "Content-Type"
	jsonType    = "application/json"
)

// prepareAndDo prepares a request for the given method, URL, and
// message body, and executes the request, returning an http.Response or an
// error.  Errors returned from this function represent http-layer errors and
// not errors in the Open Service Broker API.
func (c *client) prepareAndDo(method, URL string, params map[string]string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader

	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		bodyReader = bytes.NewReader(bodyBytes)
	}

	request, err := http.NewRequest(method, URL, bodyReader)
	if err != nil {
		return nil, err
	}

	request.Header.Set(XBrokerAPIVersion, c.APIVersion.HeaderValue())
	if bodyReader != nil {
		request.Header.Set(contentType, jsonType)
	}

	if c.BasicAuthConfig != nil {
		request.SetBasicAuth(c.BasicAuthConfig.Username, c.BasicAuthConfig.Password)
	}

	if params != nil {
		q := request.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		request.URL.RawQuery = q.Encode()
	}

	if c.Verbose {
		glog.Infof("broker %q: doing request to %q", c.Name, URL)
	}

	return c.doRequestFunc(request)
}

func (c *client) doRequest(request *http.Request) (*http.Response, error) {
	return c.httpClient.Do(request)
}

// unmarshalResponse unmartials the response body of the given response into
// the given object or returns an error.
func (c *client) unmarshalResponse(response *http.Response, obj interface{}) error {
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if c.Verbose {
		glog.Infof("broker %q: response body: %v, type: %T", c.Name, string(body), obj)
	}

	err = json.Unmarshal(body, obj)
	if err != nil {
		return err
	}

	return nil
}

// handleFailureResponse returns an HTTPStatusCodeError for the given
// response.
func (c *client) handleFailureResponse(response *http.Response) error {
	glog.Info("handling failure responses")
	brokerResponse := &failureResponseBody{}
	if err := c.unmarshalResponse(response, brokerResponse); err != nil {
		return HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
	}

	return HTTPStatusCodeError{
		StatusCode:   response.StatusCode,
		ErrorMessage: brokerResponse.Err,
		Description:  brokerResponse.Description,
	}
}

// internal message body types

type asyncSuccessResponseBody struct {
	Operation *string `json:"operation"`
}

type failureResponseBody struct {
	Err         *string `json:"error,omitempty"`
	Description *string `json:"description,omitempty"`
}
