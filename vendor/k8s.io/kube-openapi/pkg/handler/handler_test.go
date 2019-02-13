package handler

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"github.com/go-openapi/spec"
	json "github.com/json-iterator/go"
	yaml "gopkg.in/yaml.v2"
)

var returnedSwagger = []byte(`{
  "swagger": "2.0",
  "info": {
   "title": "Kubernetes",
   "version": "v1.11.0"
  }}`)

func TestRegisterOpenAPIVersionedService(t *testing.T) {
	var s spec.Swagger
	err := s.UnmarshalJSON(returnedSwagger)
	if err != nil {
		t.Errorf("Unexpected error in unmarshalling SwaggerJSON: %v", err)
	}

	returnedJSON, err := json.Marshal(s)
	if err != nil {
		t.Errorf("Unexpected error in preparing returnedJSON: %v", err)
	}
	var decodedJSON map[string]interface{}
	if err := json.Unmarshal(returnedJSON, &decodedJSON); err != nil {
		t.Fatal(err)
	}
	returnedPb, err := ToProtoBinary(decodedJSON)
	if err != nil {
		t.Errorf("Unexpected error in preparing returnedPb: %v", err)
	}

	mux := http.NewServeMux()
	o, err := NewOpenAPIService(&s)
	if err != nil {
		t.Fatal(err)
	}
	if err = o.RegisterOpenAPIVersionedService("/openapi/v2", mux); err != nil {
		t.Errorf("Unexpected error in register OpenAPI versioned service: %v", err)
	}
	server := httptest.NewServer(mux)
	defer server.Close()
	client := server.Client()

	tcs := []struct {
		acceptHeader string
		respStatus   int
		respBody     []byte
	}{
		{"", 200, returnedJSON},
		{"*/*", 200, returnedJSON},
		{"application/*", 200, returnedJSON},
		{"application/json", 200, returnedJSON},
		{"test/test", 406, []byte{}},
		{"application/test", 406, []byte{}},
		{"application/test, */*", 200, returnedJSON},
		{"application/test, application/json", 200, returnedJSON},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf", 200, returnedPb},
		{"application/json, application/com.github.proto-openapi.spec.v2@v1.0+protobuf", 200, returnedJSON},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf, application/json", 200, returnedPb},
		{"application/com.github.proto-openapi.spec.v2@v1.0+protobuf; q=0.5, application/json", 200, returnedJSON},
	}

	for _, tc := range tcs {
		req, err := http.NewRequest("GET", server.URL+"/openapi/v2", nil)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in creating new request: %v", tc.acceptHeader, err)
		}

		req.Header.Add("Accept", tc.acceptHeader)
		resp, err := client.Do(req)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in serving HTTP request: %v", tc.acceptHeader, err)
		}

		if resp.StatusCode != tc.respStatus {
			t.Errorf("Accept: %v: Unexpected response status code, want: %v, got: %v", tc.acceptHeader, tc.respStatus, resp.StatusCode)
		}
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("Accept: %v: Unexpected error in reading response body: %v", tc.acceptHeader, err)
		}
		if !reflect.DeepEqual(body, tc.respBody) {
			t.Errorf("Accept: %v: Response body mismatches, \nwant: %s, \ngot:  %s", tc.acceptHeader, string(tc.respBody), string(body))
		}
	}
}

func TestJsonToYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected yaml.MapSlice
	}{
		{"nil", nil, nil},
		{"empty", map[string]interface{}{}, yaml.MapSlice{}},
		{
			"values",
			map[string]interface{}{
				"int64":   int64(42),
				"float64": float64(42.0),
				"string":  string("foo"),
				"bool":    true,
				"slice":   []interface{}{"foo", "bar"},
				"map":     map[string]interface{}{"foo": "bar"},
			},
			yaml.MapSlice{
				{"int64", int64(42)},
				{"float64", float64(42.0)},
				{"string", string("foo")},
				{"bool", true},
				{"slice", []interface{}{"foo", "bar"}},
				{"map", yaml.MapSlice{{"foo", "bar"}}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonToYAML(tt.input)
			sortMapSlicesInPlace(tt.expected)
			sortMapSlicesInPlace(got)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("jsonToYAML() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func sortMapSlicesInPlace(x interface{}) {
	switch x := x.(type) {
	case []interface{}:
		for i := range x {
			sortMapSlicesInPlace(x[i])
		}
	case yaml.MapSlice:
		sort.Slice(x, func(a, b int) bool {
			return x[a].Key.(string) < x[b].Key.(string)
		})
	}
}

func TestToProtoBinary(t *testing.T) {
	bs, err := ioutil.ReadFile("../../test/integration/testdata/aggregator/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var j map[string]interface{}
	if err := json.Unmarshal(bs, &j); err != nil {
		t.Fatal(err)
	}
	if _, err := ToProtoBinary(j); err != nil {
		t.Fatal()
	}
	// TODO: add some kind of roundtrip test here
}
