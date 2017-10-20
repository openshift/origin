package v2

import (
	"fmt"
	"net/http"
	"testing"
)

const okCatalogBytes = `{
  "services": [{
    "name": "fake-service",
    "id": "acb56d7c-XXXX-XXXX-XXXX-feb140a59a66",
    "description": "fake service",
    "tags": ["tag1", "tag2"],
    "requires": ["route_forwarding"],
    "bindable": true,
    "metadata": {
    	"a": "b",
    	"c": "d"
    },
    "dashboard_client": {
      "id": "398e2f8e-XXXX-XXXX-XXXX-19a71ecbcf64",
      "secret": "277cabb0-XXXX-XXXX-XXXX-7822c0a90e5d",
      "redirect_uri": "http://localhost:1234"
    },
    "plan_updateable": true,
    "plans": [{
      "name": "fake-plan-1",
      "id": "d3031751-XXXX-XXXX-XXXX-a42377d3320e",
      "description": "description1",
      "metadata": {
      	"b": "c",
      	"d": "e"
      }
    }]
  }]
}`

func okCatalogResponse() *CatalogResponse {
	return &CatalogResponse{
		Services: []Service{
			{
				ID:          "acb56d7c-XXXX-XXXX-XXXX-feb140a59a66",
				Name:        "fake-service",
				Description: "fake service",
				Tags: []string{
					"tag1",
					"tag2",
				},
				Requires: []string{
					"route_forwarding",
				},
				Bindable:      true,
				PlanUpdatable: truePtr(),
				Plans: []Plan{
					{
						ID:          "d3031751-XXXX-XXXX-XXXX-a42377d3320e",
						Name:        "fake-plan-1",
						Description: "description1",
						Metadata: map[string]interface{}{
							"b": "c",
							"d": "e",
						},
					},
				},
				DashboardClient: &DashboardClient{
					ID:          "398e2f8e-XXXX-XXXX-XXXX-19a71ecbcf64",
					Secret:      "277cabb0-XXXX-XXXX-XXXX-7822c0a90e5d",
					RedirectURI: "http://localhost:1234",
				},
				Metadata: map[string]interface{}{
					"a": "b",
					"c": "d",
				},
			},
		},
	}
}

const okCatalog2Bytes = `{
  "services": [{
    "name": "fake-service-2",
    "id": "fake-service-2-id",
    "description": "service-description-2",
    "bindable": false,
    "plans": [{
      "name": "fake-plan-2",
      "id": "fake-plan-2-id",
      "description": "description-2",
      "bindable": true
    }]
  }]
}`

func okCatalog2Response() *CatalogResponse {
	return &CatalogResponse{
		Services: []Service{
			{
				ID:          "fake-service-2-id",
				Name:        "fake-service-2",
				Description: "service-description-2",
				Bindable:    false,
				Plans: []Plan{
					{
						ID:          "fake-plan-2-id",
						Name:        "fake-plan-2",
						Description: "description-2",
						Bindable:    truePtr(),
					},
				},
			},
		},
	}
}

const alphaParameterSchemaCatalogBytes = `{
  "services": [{
    "name": "fake-service",
    "id": "acb56d7c-XXXX-XXXX-XXXX-feb140a59a66",
    "description": "fake service",
    "tags": ["tag1", "tag2"],
    "requires": ["route_forwarding"],
    "bindable": true,
    "metadata": {
    	"a": "b",
    	"c": "d"
    },
    "dashboard_client": {
      "id": "398e2f8e-XXXX-XXXX-XXXX-19a71ecbcf64",
      "secret": "277cabb0-XXXX-XXXX-XXXX-7822c0a90e5d",
      "redirect_uri": "http://localhost:1234"
    },
    "plan_updateable": true,
    "plans": [{
      "name": "fake-plan-1",
      "id": "d3031751-XXXX-XXXX-XXXX-a42377d3320e",
      "description": "description1",
      "metadata": {
      	"b": "c",
      	"d": "e"
      },
      "schemas": {
      	"service_instance": {
	  	  "create": {
	  	  	"parameters": {
	  		  "foo": "bar"	
	  	  	}
	  	  },
	  	  "update": {
	  	  	"parameters": {
	  		  "baz": "zap"
	  	    }
	  	  }
      	},
      	"service_binding": {
      	  "create": {
	  	  	"parameters": {
      	  	  "zoo": "blu"
      	    }
      	  }
      	}
      }
    }]
  }]
}`

func alphaParameterCatalogResponse() *CatalogResponse {
	catalog := okCatalogResponse()
	catalog.Services[0].Plans[0].ParameterSchemas = &ParameterSchemas{
		ServiceInstances: &ServiceInstanceSchema{
			Create: &InputParameters{
				Parameters: map[string]interface{}{
					"foo": "bar",
				},
			},
			Update: &InputParameters{
				Parameters: map[string]interface{}{
					"baz": "zap",
				},
			},
		},
		ServiceBindings: &ServiceBindingSchema{
			Create: &InputParameters{
				Parameters: map[string]interface{}{
					"zoo": "blu",
				},
			},
		},
	}

	return catalog
}

func TestGetCatalog(t *testing.T) {
	cases := []struct {
		name               string
		version            APIVersion
		enableAlpha        bool
		httpReaction       httpReaction
		expectedResponse   *CatalogResponse
		expectedErrMessage string
		expectedErr        error
	}{
		{
			name: "success 1",
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   okCatalogBytes,
			},
			expectedResponse: okCatalogResponse(),
		},
		{
			name: "success 2",
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   okCatalog2Bytes,
			},
			expectedResponse: okCatalog2Response(),
		},
		{
			name: "http error",
			httpReaction: httpReaction{
				err: fmt.Errorf("http error"),
			},
			expectedErrMessage: "http error",
		},
		{
			name: "200 with malformed response",
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   malformedResponse,
			},
			expectedErrMessage: "Status: 200; ErrorMessage: <nil>; Description: <nil>; ResponseError: unexpected end of JSON input",
		},
		{
			name: "500 with malformed response",
			httpReaction: httpReaction{
				status: http.StatusInternalServerError,
				body:   malformedResponse,
			},
			expectedErrMessage: "Status: 500; ErrorMessage: <nil>; Description: <nil>; ResponseError: unexpected end of JSON input",
		},
		{
			name: "500 with conventional response",
			httpReaction: httpReaction{
				status: http.StatusInternalServerError,
				body:   conventionalFailureResponseBody,
			},
			expectedErr: testHTTPStatusCodeError(),
		},
		{
			name:    "schemas included if API version >= 2.13",
			version: Version2_13(),
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   alphaParameterSchemaCatalogBytes,
			},
			expectedResponse: alphaParameterCatalogResponse(),
		},
		{
			name:    "schemas not included if API version < 2.13",
			version: Version2_12(),
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   alphaParameterSchemaCatalogBytes,
			},
			expectedResponse: okCatalogResponse(),
		},
	}

	for _, tc := range cases {
		httpChecks := httpChecks{
			URL: "/v2/catalog",
		}

		if tc.version.label == "" {
			tc.version = Version2_11()
		}

		klient := newTestClient(t, tc.name, tc.version, tc.enableAlpha, httpChecks, tc.httpReaction)

		response, err := klient.GetCatalog()

		doResponseChecks(t, tc.name, response, err, tc.expectedResponse, tc.expectedErrMessage, tc.expectedErr)
	}
}
