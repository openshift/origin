package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emicklei/go-restful"

	"k8s.io/apiserver/pkg/authentication/user"

	openservicebrokerapi "github.com/openshift/template-service-broker/pkg/openservicebroker/api"
	client2 "github.com/openshift/template-service-broker/pkg/openservicebroker/client"
)

const validUUID = "decd59a9-1dd2-453e-942e-2deba96bfa96"

type fakeBroker openservicebrokerapi.Response

func (b *fakeBroker) Catalog() *openservicebrokerapi.Response {
	r := openservicebrokerapi.Response(*b)
	return &r
}

func (b *fakeBroker) Provision(u user.Info, instanceID string, preq *openservicebrokerapi.ProvisionRequest) *openservicebrokerapi.Response {
	r := openservicebrokerapi.Response(*b)
	return &r
}

func (b *fakeBroker) Deprovision(u user.Info, instanceID string) *openservicebrokerapi.Response {
	r := openservicebrokerapi.Response(*b)
	return &r
}

func (b *fakeBroker) Bind(u user.Info, instanceID string, bindingID string, breq *openservicebrokerapi.BindRequest) *openservicebrokerapi.Response {
	r := openservicebrokerapi.Response(*b)
	return &r
}

func (b *fakeBroker) Unbind(u user.Info, instanceID string, bindingID string) *openservicebrokerapi.Response {
	r := openservicebrokerapi.Response(*b)
	return &r
}

func (b *fakeBroker) LastOperation(u user.Info, instanceID string, operation openservicebrokerapi.Operation) *openservicebrokerapi.Response {
	r := openservicebrokerapi.Response(*b)
	return &r
}

var _ openservicebrokerapi.Broker = &fakeBroker{}

type fakeResponseWriter struct {
	h    http.Header
	code int
	buf  bytes.Buffer
	o    map[string]interface{}
}

func newFakeResponseWriter() *fakeResponseWriter {
	return &fakeResponseWriter{h: make(http.Header), code: -1}
}

func (rw *fakeResponseWriter) Header() http.Header {
	return rw.h
}

func (rw *fakeResponseWriter) Write(b []byte) (int, error) {
	if rw.code == -1 {
		rw.code = http.StatusOK
	}
	return rw.buf.Write(b)
}

func (rw *fakeResponseWriter) WriteHeader(code int) {
	rw.code = code
}

var _ http.ResponseWriter = &fakeResponseWriter{}

var defaultOriginatingIdentityHeader string

func init() {
	var err error
	defaultOriginatingIdentityHeader, err = client2.OriginatingIdentityHeader(&user.DefaultInfo{})
	if err != nil {
		panic(err)
	}
}

func parseUrl(t *testing.T, s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func checkResponseWriter(t *testing.T, rw *fakeResponseWriter) {
	expectedHeaders := map[string]string{
		restful.HEADER_ContentType:             restful.MIME_JSON,
		openservicebrokerapi.XBrokerAPIVersion: openservicebrokerapi.APIVersion,
	}
	for k, v := range expectedHeaders {
		if rw.h.Get(k) != v {
			t.Errorf("%s header was %q, expected %q", k, rw.h.Get(k), v)
		}
	}

	err := json.Unmarshal(rw.buf.Bytes(), &rw.o)
	if err != nil {
		t.Fatal(err)
	}
}

func TestRequiresXBrokerAPIVersionHeader(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.NewResponse(http.StatusOK, map[string]interface{}{}, nil))
	Route(c, "", &fb)

	rw := newFakeResponseWriter()
	c.ServeHTTP(rw, &http.Request{
		Method: http.MethodGet,
		URL:    parseUrl(t, "/v2/catalog"),
	})
	checkResponseWriter(t, rw)

	if rw.code != http.StatusPreconditionFailed {
		t.Errorf("Expected code %d, got %d", http.StatusPreconditionFailed, rw.code)
	}
	if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, "header must") {
		t.Errorf("Expected description containing text %q; got %q", "header must", description)
	}
}

func TestRequiresContentTypeHeader(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.NewResponse(http.StatusOK, map[string]interface{}{}, nil))
	Route(c, "", &fb)

	rw := newFakeResponseWriter()
	c.ServeHTTP(rw, &http.Request{
		Method: http.MethodPut,
		URL:    parseUrl(t, "/v2/service_instances/"+validUUID),
		Header: http.Header{
			openservicebrokerapi.XBrokerAPIVersion: []string{openservicebrokerapi.APIVersion},
		},
		Body: ioutil.NopCloser(bytes.NewBufferString("{}")),
	})
	checkResponseWriter(t, rw)

	if rw.code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected code %d, got %d", http.StatusUnsupportedMediaType, rw.code)
	}
	if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, "header must") {
		t.Errorf("Expected description containing text %q; got %q", "header must", description)
	}
}

func TestInternalServerError(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.InternalServerError(errors.New("test error")))
	Route(c, "", &fb)

	rw := newFakeResponseWriter()
	c.ServeHTTP(rw, &http.Request{
		Method: http.MethodGet,
		URL:    parseUrl(t, "/v2/catalog"),
		Header: http.Header{openservicebrokerapi.XBrokerAPIVersion: []string{openservicebrokerapi.APIVersion}},
	})
	checkResponseWriter(t, rw)

	if rw.code != http.StatusInternalServerError {
		t.Errorf("Expected code %d, got %d", http.StatusInternalServerError, rw.code)
	}
	if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, "test error") {
		t.Errorf("Expected description containing text %q", "test error")
	}
}

func TestBadRequestError(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.BadRequest(errors.New("test error")))
	Route(c, "", &fb)

	rw := newFakeResponseWriter()
	c.ServeHTTP(rw, &http.Request{
		Method: http.MethodGet,
		URL:    parseUrl(t, "/v2/catalog"),
		Header: http.Header{openservicebrokerapi.XBrokerAPIVersion: []string{openservicebrokerapi.APIVersion}},
	})
	checkResponseWriter(t, rw)

	if rw.code != http.StatusBadRequest {
		t.Errorf("Expected code %d, got %d", http.StatusBadRequest, rw.code)
	}
	if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, "test error") {
		t.Errorf("Expected description containing text %q", "test error")
	}
}

func TestProvision(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.NewResponse(http.StatusOK, map[string]interface{}{}, nil))
	Route(c, "", &fb)

	tests := []struct {
		name        string
		req         http.Request
		body        *openservicebrokerapi.ProvisionRequest
		expectCode  int
		expectError string
	}{
		{
			name: "bad instance_id",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/bad"),
			},
			expectError: `instance_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "empty body",
			req: http.Request{
				URL:  parseUrl(t, "/v2/service_instances/"+validUUID),
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			},
			expectError: `EOF`,
		},
		{
			name: "bad body",
			req: http.Request{
				URL:  parseUrl(t, "/v2/service_instances/"+validUUID),
				Body: ioutil.NopCloser(bytes.NewBufferString("bad")),
			},
			expectError: `invalid character`,
		},
		{
			name: "invalid body",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID),
			},
			body:        &openservicebrokerapi.ProvisionRequest{},
			expectError: `service_id: Invalid value: "": must be a valid UUID`,
		},
		{
			name: "no acceptsincomplete",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID),
			},
			body: &openservicebrokerapi.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: openservicebrokerapi.KubernetesContext{
					Platform:  openservicebrokerapi.ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectCode:  http.StatusUnprocessableEntity,
			expectError: `This request requires client support for asynchronous service operations.`,
		},
		{
			name: "no identity",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"?accepts_incomplete=true"),
			},
			body: &openservicebrokerapi.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: openservicebrokerapi.KubernetesContext{
					Platform:  openservicebrokerapi.ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectCode:  http.StatusBadRequest,
			expectError: "couldn't parse X-Broker-API-Originating-Identity header",
		},
		{
			name: "good",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"?accepts_incomplete=true"),
				Header: http.Header{
					http.CanonicalHeaderKey(openservicebrokerapi.XBrokerAPIOriginatingIdentity): []string{defaultOriginatingIdentityHeader},
				},
			},
			body: &openservicebrokerapi.ProvisionRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
				Context: openservicebrokerapi.KubernetesContext{
					Platform:  openservicebrokerapi.ContextPlatformKubernetes,
					Namespace: "test",
				},
			},
			expectCode: http.StatusOK,
		},
	}

	for _, test := range tests {
		rw := newFakeResponseWriter()

		test.req.Method = http.MethodPut
		if test.req.Header == nil {
			test.req.Header = make(http.Header)
		}
		test.req.Header.Set(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)
		test.req.Header.Set(restful.HEADER_ContentType, restful.MIME_JSON)
		if test.expectCode == 0 {
			test.expectCode = http.StatusBadRequest
		}

		if test.body != nil {
			b, err := json.Marshal(&test.body)
			if err != nil {
				t.Fatal(err)
			}
			test.req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		}

		c.ServeHTTP(rw, &test.req)
		checkResponseWriter(t, rw)

		if test.expectCode != rw.code {
			t.Errorf("%q: expectCode was %d but code was %d", test.name, test.expectCode, rw.code)
		}
		if test.expectError == "" {
			if description, ok := rw.o["description"].(string); ok {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		} else {
			if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, test.expectError) {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		}
	}
}

func TestDeprovision(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.NewResponse(http.StatusOK, map[string]interface{}{}, nil))
	Route(c, "", &fb)

	tests := []struct {
		name        string
		req         http.Request
		expectCode  int
		expectError string
	}{
		{
			name: "bad instance_id",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/bad"),
			},
			expectError: `instance_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "no acceptsincomplete",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID),
			},
			expectCode:  http.StatusUnprocessableEntity,
			expectError: `This request requires client support for asynchronous service operations.`,
		},
		{
			name: "no identity",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"?accepts_incomplete=true"),
			},
			expectCode:  http.StatusBadRequest,
			expectError: "couldn't parse X-Broker-API-Originating-Identity header",
		},
		{
			name: "good",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"?accepts_incomplete=true"),
				Header: http.Header{
					http.CanonicalHeaderKey(openservicebrokerapi.XBrokerAPIOriginatingIdentity): []string{defaultOriginatingIdentityHeader},
				},
			},
			expectCode: http.StatusOK,
		},
	}

	for _, test := range tests {
		rw := newFakeResponseWriter()

		test.req.Method = http.MethodDelete
		if test.req.Header == nil {
			test.req.Header = make(http.Header)
		}
		test.req.Header.Set(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)
		if test.expectCode == 0 {
			test.expectCode = http.StatusBadRequest
		}

		c.ServeHTTP(rw, &test.req)
		checkResponseWriter(t, rw)

		if test.expectCode != rw.code {
			t.Errorf("%q: expectCode was %d but code was %d", test.name, test.expectCode, rw.code)
		}
		if test.expectError == "" {
			if description, ok := rw.o["description"].(string); ok {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		} else {
			if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, test.expectError) {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		}
	}
}

func TestLastOperation(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.NewResponse(http.StatusOK, map[string]interface{}{}, nil))
	Route(c, "", &fb)

	tests := []struct {
		name        string
		req         http.Request
		expectCode  int
		expectError string
	}{
		{
			name: "bad instance_id",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/bad/last_operation"),
			},
			expectError: `instance_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "no operation",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/last_operation"),
			},
			expectError: `invalid operation`,
		},
		{
			name: "no identity",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/last_operation?operation=provisioning"),
			},
			expectCode:  http.StatusBadRequest,
			expectError: "couldn't parse X-Broker-API-Originating-Identity header",
		},
		{
			name: "good",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/last_operation?operation=provisioning"),
				Header: http.Header{
					http.CanonicalHeaderKey(openservicebrokerapi.XBrokerAPIOriginatingIdentity): []string{defaultOriginatingIdentityHeader},
				},
			},
			expectCode: http.StatusOK,
		},
	}

	for _, test := range tests {
		rw := newFakeResponseWriter()

		test.req.Method = http.MethodGet
		if test.req.Header == nil {
			test.req.Header = make(http.Header)
		}
		test.req.Header.Set(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)
		if test.expectCode == 0 {
			test.expectCode = http.StatusBadRequest
		}

		c.ServeHTTP(rw, &test.req)
		checkResponseWriter(t, rw)

		if test.expectCode != rw.code {
			t.Errorf("%q: expectCode was %d but code was %d", test.name, test.expectCode, rw.code)
		}
		if test.expectError == "" {
			if description, ok := rw.o["description"].(string); ok {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		} else {
			if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, test.expectError) {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		}
	}
}

func TestBind(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.NewResponse(http.StatusOK, map[string]interface{}{}, nil))
	Route(c, "", &fb)

	tests := []struct {
		name        string
		req         http.Request
		body        *openservicebrokerapi.BindRequest
		expectCode  int
		expectError string
	}{
		{
			name: "bad instance_id",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/bad/service_bindings/"+validUUID),
			},
			expectError: `instance_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "bad binding_id",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/bad"),
			},
			expectError: `binding_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "empty body",
			req: http.Request{
				URL:  parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/"+validUUID),
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			},
			expectError: `EOF`,
		},
		{
			name: "bad body",
			req: http.Request{
				URL:  parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/"+validUUID),
				Body: ioutil.NopCloser(bytes.NewBufferString("bad")),
			},
			expectError: `invalid character`,
		},
		{
			name: "invalid body",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/"+validUUID),
			},
			body:        &openservicebrokerapi.BindRequest{},
			expectError: `service_id: Invalid value: "": must be a valid UUID`,
		},
		{
			name: "no identity",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/"+validUUID),
			},
			body: &openservicebrokerapi.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
			},
			expectCode:  http.StatusBadRequest,
			expectError: "couldn't parse X-Broker-API-Originating-Identity header",
		},
		{
			name: "good",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/"+validUUID),
				Header: http.Header{
					http.CanonicalHeaderKey(openservicebrokerapi.XBrokerAPIOriginatingIdentity): []string{defaultOriginatingIdentityHeader},
				},
			},
			body: &openservicebrokerapi.BindRequest{
				ServiceID: validUUID,
				PlanID:    validUUID,
			},
			expectCode: http.StatusOK,
		},
	}

	for _, test := range tests {
		rw := newFakeResponseWriter()

		test.req.Method = http.MethodPut
		if test.req.Header == nil {
			test.req.Header = make(http.Header)
		}
		test.req.Header.Set(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)
		test.req.Header.Set(restful.HEADER_ContentType, restful.MIME_JSON)
		if test.expectCode == 0 {
			test.expectCode = http.StatusBadRequest
		}

		if test.body != nil {
			b, err := json.Marshal(&test.body)
			if err != nil {
				t.Fatal(err)
			}
			test.req.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		}

		c.ServeHTTP(rw, &test.req)
		checkResponseWriter(t, rw)

		if test.expectCode != rw.code {
			t.Errorf("%q: expectCode was %d but code was %d", test.name, test.expectCode, rw.code)
		}
		if test.expectError == "" {
			if description, ok := rw.o["description"].(string); ok {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		} else {
			if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, test.expectError) {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		}
	}
}

func TestUnbind(t *testing.T) {
	c := restful.NewContainer()
	fb := fakeBroker(*openservicebrokerapi.NewResponse(http.StatusOK, map[string]interface{}{}, nil))
	Route(c, "", &fb)

	tests := []struct {
		name        string
		req         http.Request
		expectCode  int
		expectError string
	}{
		{
			name: "bad instance_id",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/bad/service_bindings/"+validUUID),
			},
			expectError: `instance_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "bad binding_id",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/bad"),
			},
			expectError: `binding_id: Invalid value: "bad": must be a valid UUID`,
		},
		{
			name: "no identity",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/"+validUUID),
			},
			expectCode:  http.StatusBadRequest,
			expectError: "couldn't parse X-Broker-API-Originating-Identity header",
		},
		{
			name: "good",
			req: http.Request{
				URL: parseUrl(t, "/v2/service_instances/"+validUUID+"/service_bindings/"+validUUID),
				Header: http.Header{
					http.CanonicalHeaderKey(openservicebrokerapi.XBrokerAPIOriginatingIdentity): []string{defaultOriginatingIdentityHeader},
				},
			},
			expectCode: http.StatusOK,
		},
	}

	for _, test := range tests {
		rw := newFakeResponseWriter()

		test.req.Method = http.MethodDelete
		if test.req.Header == nil {
			test.req.Header = make(http.Header)
		}
		test.req.Header.Set(openservicebrokerapi.XBrokerAPIVersion, openservicebrokerapi.APIVersion)
		test.req.Header.Set(restful.HEADER_ContentType, restful.MIME_JSON)
		if test.expectCode == 0 {
			test.expectCode = http.StatusBadRequest
		}

		c.ServeHTTP(rw, &test.req)
		checkResponseWriter(t, rw)

		if test.expectCode != rw.code {
			t.Errorf("%q: expectCode was %d but code was %d", test.name, test.expectCode, rw.code)
		}
		if test.expectError == "" {
			if description, ok := rw.o["description"].(string); ok {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		} else {
			if description, ok := rw.o["description"].(string); !ok || !strings.Contains(description, test.expectError) {
				t.Errorf("%q: expectError was %q but description was %q", test.name, test.expectError, description)
			}
		}
	}
}
