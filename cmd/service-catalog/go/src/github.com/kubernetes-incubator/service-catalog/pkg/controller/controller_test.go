/*
Copyright 2017 The Kubernetes Authors.

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

package controller

import (
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	fakeosb "github.com/pmorie/go-open-service-broker-client/v2/fake"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	servicecataloginformers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions"
	v1alpha1informers "github.com/kubernetes-incubator/service-catalog/pkg/client/informers_generated/externalversions/servicecatalog/v1alpha1"

	servicecatalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"

	fakebrokerserver "github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/fake/server"
	"k8s.io/apimachinery/pkg/api/meta"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
)

// NOTE:
//
// This file contains:
//
// - tests for the methods on controller.go
// - test fixtures used in other controller_*_test.go files
//
// Other controller_*_test.go files contain tests related to the reconcilation
// loops for the different catalog API resources.

const (
	serviceClassGUID            = "SCGUID"
	planGUID                    = "PGUID"
	nonbindableServiceClassGUID = "UNBINDABLE-SERVICE"
	nonbindablePlanGUID         = "UNBINDABLE-PLAN"
	instanceGUID                = "IGUID"
	bindingGUID                 = "BGUID"

	testBrokerName                  = "test-broker"
	testServiceClassName            = "test-serviceclass"
	testNonbindableServiceClassName = "test-unbindable-serviceclass"
	testPlanName                    = "test-plan"
	testNonbindablePlanName         = "test-unbindable-plan"
	testInstanceName                = "test-instance"
	testBindingName                 = "test-binding"
	testNamespace                   = "test-ns"
	testBindingSecretName           = "test-secret"
	testOperation                   = "test-operation"
	testNsUID                       = "test-ns-uid"
)

var testDashboardURL = "http://dashboard"

const testCatalog = `{
  "services": [{
    "name": "fake-service",
    "id": "acb56d7c-XXXX-XXXX-XXXX-feb140a59a66",
    "description": "fake service",
    "tags": ["no-sql", "relational"],
    "requires": ["route_forwarding"],
    "max_db_per_node": 5,
    "bindable": true,
    "metadata": {
      "provider": {
        "name": "The name"
      },
      "listing": {
        "imageUrl": "http://example.com/cat.gif",
        "blurb": "Add a blurb here",
        "longDescription": "A long time ago, in a galaxy far far away..."
      },
      "displayName": "The Fake Broker"
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
      "description": "Shared fake Server, 5tb persistent disk, 40 max concurrent connections",
      "max_storage_tb": 5,
      "metadata": {
        "costs":[
            {
               "amount":{
                  "usd":99.0
               },
               "unit":"MONTHLY"
            },
            {
               "amount":{
                  "usd":0.99
               },
               "unit":"1GB of messages over 20GB"
            }
         ],
        "bullets": [
            "Shared fake server",
            "5 TB storage",
            "40 concurrent connections"
        ]
      }
    }, {
      "name": "fake-plan-2",
      "id": "0f4008b5-XXXX-XXXX-XXXX-dace631cd648",
      "description": "Shared fake Server, 5tb persistent disk, 40 max concurrent connections. 100 async",
      "max_storage_tb": 5,
      "metadata": {
        "costs":[
            {
               "amount":{
                  "usd":199.0
               },
               "unit":"MONTHLY"
            },
            {
               "amount":{
                  "usd":0.99
               },
               "unit":"1GB of messages over 20GB"
            }
         ],
        "bullets": [
          "40 concurrent connections"
        ]
      }
    }]
  }]
}`

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
	          "$schema": "http://json-schema.org/draft-04/schema",
	          "type": "object",
	          "title": "Parameters",
	          "properties": {
	            "name": {
	              "title": "Queue Name",
	              "type": "string",
	              "maxLength": 63,
	              "default": "My Queue"
	            },
	            "email": {
	              "title": "Email",
	              "type": "string",
	              "pattern": "^\\S+@\\S+$",
	              "description": "Email address for alerts."
	            },
	            "protocol": {
	              "title": "Protocol",
	              "type": "string",
	              "default": "Java Message Service (JMS) 1.1",
	              "enum": [
	                "Java Message Service (JMS) 1.1",
	                "Transmission Control Protocol (TCP)",
	                "Advanced Message Queuing Protocol (AMQP) 1.0"
	              ]
	            },
	            "secure": {
	              "title": "Enable security",
	              "type": "boolean",
	              "default": true
	            }
	          },
	          "required": [
	            "name",
	            "protocol"
	          ]
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

const instanceParameterSchemaBytes = `{
  "$schema": "http://json-schema.org/draft-04/schema",
  "type": "object",
  "title": "Parameters",
  "properties": {
    "name": {
      "title": "Queue Name",
      "type": "string",
      "maxLength": 63,
      "default": "My Queue"
    },
    "email": {
      "title": "Email",
      "type": "string",
      "pattern": "^\\S+@\\S+$",
      "description": "Email address for alerts."
    },
    "protocol": {
      "title": "Protocol",
      "type": "string",
      "default": "Java Message Service (JMS) 1.1",
      "enum": [
        "Java Message Service (JMS) 1.1",
        "Transmission Control Protocol (TCP)",
        "Advanced Message Queuing Protocol (AMQP) 1.0"
      ]
    },
    "secure": {
      "title": "Enable security",
      "type": "boolean",
      "default": true
    }
  },
  "required": [
    "name",
    "protocol"
  ]
}`

// broker used in most of the tests that need a broker
func getTestBroker() *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{Name: testBrokerName},
		Spec: v1alpha1.BrokerSpec{
			URL: "https://example.com",
		},
	}
}

func getTestBrokerWithStatus(status v1alpha1.ConditionStatus) *v1alpha1.Broker {
	broker := getTestBroker()
	broker.Status = v1alpha1.BrokerStatus{
		Conditions: []v1alpha1.BrokerCondition{{
			Type:               v1alpha1.BrokerConditionReady,
			Status:             status,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
	}

	return broker
}

// a bindable service class wired to the result of getTestBroker()
func getTestServiceClass() *v1alpha1.ServiceClass {
	return &v1alpha1.ServiceClass{
		ObjectMeta:  metav1.ObjectMeta{Name: testServiceClassName},
		BrokerName:  testBrokerName,
		Description: "a test service",
		ExternalID:  serviceClassGUID,
		Bindable:    true,
		Plans: []v1alpha1.ServicePlan{
			{
				Name:        testPlanName,
				Description: "a test plan",
				Free:        true,
				ExternalID:  planGUID,
			},
			{
				Name:        testNonbindablePlanName,
				Description: "a test plan",
				Free:        true,
				ExternalID:  nonbindablePlanGUID,
				Bindable:    falsePtr(),
			},
		},
	}
}

// an unbindable service class wired to the result of getTestBroker()
func getTestNonbindableServiceClass() *v1alpha1.ServiceClass {
	return &v1alpha1.ServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: testNonbindableServiceClassName},
		BrokerName: testBrokerName,
		ExternalID: nonbindableServiceClassGUID,
		Bindable:   false,
		Plans: []v1alpha1.ServicePlan{
			{
				Name:       testPlanName,
				Free:       true,
				ExternalID: planGUID,
				Bindable:   truePtr(),
			},
			{
				Name:       testNonbindablePlanName,
				Free:       true,
				ExternalID: nonbindablePlanGUID,
				Bindable:   falsePtr(),
			},
		},
	}
}

// broker catalog that provides the service class named in of
// getTestServiceClass()
func getTestCatalog() *osb.CatalogResponse {
	return &osb.CatalogResponse{
		Services: []osb.Service{
			{
				Name:        testServiceClassName,
				ID:          serviceClassGUID,
				Description: "a test service",
				Bindable:    true,
				Plans: []osb.Plan{
					{
						Name:        testPlanName,
						Free:        truePtr(),
						ID:          planGUID,
						Description: "a test plan",
					},
					{
						Name:        testNonbindablePlanName,
						Free:        truePtr(),
						ID:          nonbindablePlanGUID,
						Description: "a test plan",
						Bindable:    falsePtr(),
					},
				},
			},
		},
	}
}

// instance referencing the result of getTestServiceClass()
func getTestInstance() *v1alpha1.Instance {
	return &v1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: testInstanceName, Namespace: testNamespace},
		Spec: v1alpha1.InstanceSpec{
			ServiceClassName: testServiceClassName,
			PlanName:         testPlanName,
			ExternalID:       instanceGUID,
		},
	}
}

// an instance referencing the result of getTestNonbindableServiceClass, on the non-bindable plan.
func getTestNonbindableInstance() *v1alpha1.Instance {
	i := getTestInstance()
	i.Spec.ServiceClassName = testNonbindableServiceClassName
	i.Spec.PlanName = testNonbindablePlanName

	return i
}

// an instance referencing the result of getTestNonbindableServiceClass, on the bindable plan.
func getTestInstanceNonbindableServiceBindablePlan() *v1alpha1.Instance {
	i := getTestNonbindableInstance()
	i.Spec.PlanName = testPlanName

	return i
}

func getTestInstanceBindableServiceNonbindablePlan() *v1alpha1.Instance {
	i := getTestInstance()
	i.Spec.PlanName = testNonbindablePlanName

	return i
}

func getTestInstanceWithStatus(status v1alpha1.ConditionStatus) *v1alpha1.Instance {
	instance := getTestInstance()
	instance.Status = v1alpha1.InstanceStatus{
		Conditions: []v1alpha1.InstanceCondition{{
			Type:               v1alpha1.InstanceConditionReady,
			Status:             status,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
	}

	return instance
}

// getTestInstanceAsync returns an instance in async mode
func getTestInstanceAsyncProvisioning(operation string) *v1alpha1.Instance {
	instance := getTestInstance()
	if operation != "" {
		instance.Status.LastOperation = &operation
	}
	instance.Status = v1alpha1.InstanceStatus{
		Conditions: []v1alpha1.InstanceCondition{{
			Type:               v1alpha1.InstanceConditionReady,
			Status:             v1alpha1.ConditionFalse,
			Message:            "Provisioning",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
		AsyncOpInProgress: true,
	}

	return instance
}

func getTestInstanceAsyncDeprovisioning(operation string) *v1alpha1.Instance {
	instance := getTestInstance()
	if operation != "" {
		instance.Status.LastOperation = &operation
	}
	instance.Status = v1alpha1.InstanceStatus{
		Conditions: []v1alpha1.InstanceCondition{{
			Type:               v1alpha1.InstanceConditionReady,
			Status:             v1alpha1.ConditionFalse,
			Message:            "Deprovisioning",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
		AsyncOpInProgress: true,
	}

	// Set the deleted timestamp to simulate deletion
	ts := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	instance.DeletionTimestamp = &ts
	return instance
}

func getTestInstanceAsyncDeprovisioningWithFinalizer(operation string) *v1alpha1.Instance {
	instance := getTestInstanceAsyncDeprovisioning(operation)
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	return instance
}

// binding referencing the result of getTestInstance()
func getTestBinding() *v1alpha1.Binding {
	return &v1alpha1.Binding{
		ObjectMeta: metav1.ObjectMeta{Name: testBindingName, Namespace: testNamespace},
		Spec: v1alpha1.BindingSpec{
			InstanceRef: v1.LocalObjectReference{Name: testInstanceName},
			ExternalID:  bindingGUID,
		},
	}
}

type instanceParameters struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args"`
}

type bindingParameters struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

func TestEmptyCatalogConversion(t *testing.T) {
	serviceClasses, err := convertCatalog(&osb.CatalogResponse{})
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 0 {
		t.Fatalf("Expected 0 serviceclasses for empty catalog, but got: %d", len(serviceClasses))
	}
}

func TestCatalogConversion(t *testing.T) {
	catalog := &osb.CatalogResponse{}
	err := json.Unmarshal([]byte(testCatalog), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}
	serviceClasses, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 1 {
		t.Fatalf("Expected 1 serviceclasses for testCatalog, but got: %d", len(serviceClasses))
	}
	serviceClass := serviceClasses[0]
	if len(serviceClass.Plans) != 2 {
		t.Fatalf("Expected 2 plans for testCatalog, but got: %d", len(serviceClass.Plans))
	}

	checkPlan(serviceClass, 0, "fake-plan-1", "Shared fake Server, 5tb persistent disk, 40 max concurrent connections", t)
	checkPlan(serviceClass, 1, "fake-plan-2", "Shared fake Server, 5tb persistent disk, 40 max concurrent connections. 100 async", t)
}

func TestCatalogConversionWithAlphaParameterSchemas(t *testing.T) {
	catalog := &osb.CatalogResponse{}
	err := json.Unmarshal([]byte(alphaParameterSchemaCatalogBytes), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}
	serviceClasses, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 1 {
		t.Fatalf("Expected 1 serviceclasses for testCatalog, but got: %d", len(serviceClasses))
	}
	serviceClass := serviceClasses[0]
	if len(serviceClass.Plans) != 1 {
		t.Fatalf("Expected 1 plan for testCatalog, but got: %d", len(serviceClass.Plans))
	}

	plan := serviceClass.Plans[0]
	if plan.AlphaInstanceCreateParameterSchema == nil {
		t.Fatalf("Expected plan.AlphaInstanceCreateParameterSchema to be set, but was nil")
	}

	cSchema := make(map[string]interface{})
	if err := json.Unmarshal(plan.AlphaInstanceCreateParameterSchema.Raw, &cSchema); err == nil {
		schema := make(map[string]interface{})
		if err := json.Unmarshal([]byte(instanceParameterSchemaBytes), &schema); err != nil {
			t.Fatalf("Error unmarshalling schema bytes: %v", err)
		}

		if e, a := schema, cSchema; !reflect.DeepEqual(e, a) {
			t.Fatalf("Unexpected value of alphaInstanceCreateParameterSchema; expected %v, got %v", e, a)
		}
	}

	if plan.AlphaInstanceUpdateParameterSchema == nil {
		t.Fatalf("Expected plan.AlphaInstanceUpdateParameterSchema to be set, but was nil")
	}
	m := make(map[string]string)
	if err := json.Unmarshal(plan.AlphaInstanceUpdateParameterSchema.Raw, &m); err == nil {
		if e, a := "zap", m["baz"]; e != a {
			t.Fatalf("Unexpected value of alphaInstanceUpdateParameterSchema; expected %v, got %v", e, a)
		}
	}

	if plan.AlphaBindingCreateParameterSchema == nil {
		t.Fatalf("Expected plan.AlphaBindingCreateParameterSchema to be set, but was nil")
	}
	m = make(map[string]string)
	if err := json.Unmarshal(plan.AlphaBindingCreateParameterSchema.Raw, &m); err == nil {
		if e, a := "blu", m["zoo"]; e != a {
			t.Fatalf("Unexpected value of alphaBindingCreateParameterSchema; expected %v, got %v", e, a)
		}
	}
}

func checkPlan(serviceClass *v1alpha1.ServiceClass, index int, planName, planDescription string, t *testing.T) {
	plan := serviceClass.Plans[index]
	if plan.Name != planName {
		t.Fatalf("Expected plan %d's name to be \"%s\", but was: %s", index, planName, plan.Name)
	}
	if plan.Description != planDescription {
		t.Fatalf("Expected plan %d's description to be \"%s\", but was: %s", index, planDescription, plan.Description)
	}
}

const testCatalogWithMultipleServices = `{
  "services": [
    {
      "name": "service1",
      "description": "service 1 description",
      "metadata": {
        "field1": "value1"
      },
      "plans": [{
        "name": "s1plan1",
        "id": "s1_plan1_id",
        "description": "s1 plan1 description"
      },
      {
        "name": "s1plan2",
        "id": "s1_plan2_id",
        "description": "s1 plan2 description"
      }]
    },
    {
      "name": "service2",
      "description": "service 2 description",
      "plans": [{
        "name": "s2plan1",
        "id": "s2_plan1_id",
        "description": "s2 plan1 description"
      },
      {
        "name": "s2plan2",
        "id": "s2_plan2_id",
        "description": "s2 plan2 description"
      }]
    }
]}`

// FIX: there is an inconsistency between the current broker API types re: the
// Service.Metadata field.  Our repo types it as `interface{}`, the go-open-
// service-broker-client types it as `map[string]interface{}`.
func TestCatalogConversionMultipleServiceClasses(t *testing.T) {
	// catalog := &osb.CatalogResponse{}
	// err := json.Unmarshal([]byte(testCatalogWithMultipleServices), &catalog)
	// if err != nil {
	// 	t.Fatalf("Failed to unmarshal test catalog: %v", err)
	// }

	// serviceClasses, err := convertCatalog(catalog)
	// if err != nil {
	// 	t.Fatalf("Failed to convertCatalog: %v", err)
	// }
	// if len(serviceClasses) != 2 {
	// 	t.Fatalf("Expected 2 serviceclasses for empty catalog, but got: %d", len(serviceClasses))
	// }
	// foundSvcMeta1 := false
	// // foundSvcMeta2 := false
	// // foundPlanMeta := false
	// for _, sc := range serviceClasses {
	// 	// For service1 make sure we have service level metadata with field1 = value1 as the blob
	// 	// and for service1 plan s1plan2 we have planmeta = planvalue as the blob.
	// 	if sc.Name == "service1" {
	// 		if sc.Description != "service 1 description" {
	// 			t.Fatalf("Expected service1's description to be \"service 1 description\", but was: %s", sc.Description)
	// 		}
	// 		if sc.ExternalMetadata != nil && len(sc.ExternalMetadata.Raw) > 0 {
	// 			m := make(map[string]string)
	// 			if err := json.Unmarshal(sc.ExternalMetadata.Raw, &m); err == nil {
	// 				if m["field1"] == "value1" {
	// 					foundSvcMeta1 = true
	// 				}
	// 			}

	// 		}
	// 		if len(sc.Plans) != 2 {
	// 			t.Fatalf("Expected 2 plans for service1 but got: %d", len(sc.Plans))
	// 		}
	// 		for _, sp := range sc.Plans {
	// 			if sp.Name == "s1plan2" {
	// 				if sp.ExternalMetadata != nil && len(sp.ExternalMetadata.Raw) > 0 {
	// 					m := make(map[string]string)
	// 					if err := json.Unmarshal(sp.ExternalMetadata.Raw, &m); err != nil {
	// 						t.Fatalf("Failed to unmarshal plan metadata: %s: %v", string(sp.ExternalMetadata.Raw), err)
	// 					}
	// 					if m["planmeta"] == "planvalue" {
	// 						foundPlanMeta = true
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// 	// For service2 make sure we have service level metadata with three element array with elements
	// 	// "first", "second", and "third"
	// 	if sc.Name == "service2" {
	// 		if sc.Description != "service 2 description" {
	// 			t.Fatalf("Expected service2's description to be \"service 2 description\", but was: %s", sc.Description)
	// 		}
	// 		if sc.ExternalMetadata != nil && len(sc.ExternalMetadata.Raw) > 0 {
	// 			m := make([]string, 0)
	// 			if err := json.Unmarshal(sc.ExternalMetadata.Raw, &m); err != nil {
	// 				t.Fatalf("Failed to unmarshal service metadata: %s: %v", string(sc.ExternalMetadata.Raw), err)
	// 			}
	// 			if len(m) != 3 {
	// 				t.Fatalf("Expected 3 fields in metadata, but got %d", len(m))
	// 			}
	// 			foundFirst := false
	// 			foundSecond := false
	// 			foundThird := false
	// 			for _, e := range m {
	// 				if e == "first" {
	// 					foundFirst = true
	// 				}
	// 				if e == "second" {
	// 					foundSecond = true
	// 				}
	// 				if e == "third" {
	// 					foundThird = true
	// 				}
	// 			}
	// 			if !foundFirst {
	// 				t.Fatalf("Didn't find 'first' in plan metadata")
	// 			}
	// 			if !foundSecond {
	// 				t.Fatalf("Didn't find 'second' in plan metadata")
	// 			}
	// 			if !foundThird {
	// 				t.Fatalf("Didn't find 'third' in plan metadata")
	// 			}
	// 			foundSvcMeta2 = true
	// 		}
	// 	}
	// }
	// if !foundSvcMeta1 {
	// 	t.Fatalf("Didn't find metadata in service1")
	// }
	// if !foundSvcMeta2 {
	// 	t.Fatalf("Didn't find metadata in service2")
	// }
	// if !foundPlanMeta {
	// 	t.Fatalf("Didn't find metadata '' in service1 plan2")
	// }

}

const testCatalogForServicePlanBindableOverride = `{
  "services": [
    {
      "name": "bindable",
      "bindable": true,
      "plans": [{
        "name": "bindable-bindable",
        "id": "s1_plan1_id"
      },
      {
        "name": "bindable-unbindable",
        "id": "s1_plan2_id",
        "bindable": false
      }]
    },
    {
      "name": "unbindable",
      "bindable": false,
      "plans": [{
        "name": "unbindable-unbindable",
        "id": "s2_plan1_id"
      },
      {
        "name": "unbindable-bindable",
        "id": "s2_plan2_id",
        "bindable": true
      }]
    }
]}`

func truePtr() *bool {
	b := true
	return &b
}

func falsePtr() *bool {
	b := false
	return &b
}

func strPtr(s string) *string {
	return &s
}

func TestCatalogConversionServicePlanBindable(t *testing.T) {
	catalog := &osb.CatalogResponse{}
	err := json.Unmarshal([]byte(testCatalogForServicePlanBindableOverride), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}

	actual, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}

	expected := []*v1alpha1.ServiceClass{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bindable",
			},
			Bindable: true,
			Plans: []v1alpha1.ServicePlan{
				{
					Name:       "bindable-bindable",
					ExternalID: "s1_plan1_id",
				},
				{
					Name:       "bindable-unbindable",
					ExternalID: "s1_plan2_id",
					Bindable:   falsePtr(),
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "unbindable",
			},
			Bindable: false,
			Plans: []v1alpha1.ServicePlan{
				{
					Name:       "unbindable-unbindable",
					ExternalID: "s2_plan1_id",
				},
				{
					Name:       "unbindable-bindable",
					ExternalID: "s2_plan2_id",
					Bindable:   truePtr(),
				},
			},
		},
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("Unexpected diff between expected and actual catalogs: %v", diff.ObjectReflectDiff(expected, actual))
	}
}

func TestIsBrokerReady(t *testing.T) {
	cases := []struct {
		name  string
		input *v1alpha1.Instance
		ready bool
	}{
		{
			name:  "ready",
			input: getTestInstanceWithStatus(v1alpha1.ConditionTrue),
			ready: true,
		},
		{
			name:  "no status",
			input: getTestInstance(),
			ready: false,
		},
		{
			name:  "not ready",
			input: getTestInstanceWithStatus(v1alpha1.ConditionFalse),
			ready: false,
		},
	}

	for _, tc := range cases {
		if e, a := tc.ready, isInstanceReady(tc.input); e != a {
			t.Errorf("%v: expected result %v, got %v", tc.name, e, a)
		}
	}
}

func TestIsPlanBindable(t *testing.T) {
	serviceClass := func(bindable bool) *v1alpha1.ServiceClass {
		serviceClass := getTestServiceClass()
		serviceClass.Bindable = bindable
		return serviceClass
	}

	servicePlan := func(bindable *bool) *v1alpha1.ServicePlan {
		return &v1alpha1.ServicePlan{
			Bindable: bindable,
		}
	}

	cases := []struct {
		name         string
		serviceClass bool
		servicePlan  *bool
		bindable     bool
	}{
		{
			name:         "service true, plan not set",
			serviceClass: true,
			bindable:     true,
		},
		{
			name:         "service true, plan false",
			serviceClass: true,
			servicePlan:  falsePtr(),
			bindable:     false,
		},
		{
			name:         "service true, plan true",
			serviceClass: true,
			servicePlan:  truePtr(),
			bindable:     true,
		},
		{
			name:         "service false, plan not set",
			serviceClass: false,
			bindable:     false,
		},
		{
			name:         "service false, plan false",
			serviceClass: false,
			servicePlan:  falsePtr(),
			bindable:     false,
		},
		{
			name:         "service false, plan true",
			serviceClass: false,
			servicePlan:  truePtr(),
			bindable:     true,
		},
	}

	for _, tc := range cases {
		sc := serviceClass(tc.serviceClass)
		plan := servicePlan(tc.servicePlan)

		if e, a := tc.bindable, isPlanBindable(sc, plan); e != a {
			t.Errorf("%v: unexpected result; expected %v, got %v", tc.name, e, a)
		}
	}
}

// newTestController creates a new test controller injected with fake clients
// and returns:
//
// - a fake kubernetes core api client
// - a fake service catalog api client
// - a fake osb client
// - a test controller
// - the shared informers for the service catalog v1alpha1 api
//
// If there is an error, newTestController calls 'Fatal' on the injected
// testing.T.
func newTestController(t *testing.T, config fakeosb.FakeClientConfiguration) (
	*clientgofake.Clientset,
	*servicecatalogclientset.Clientset,
	*fakeosb.FakeClient,
	*controller,
	v1alpha1informers.Interface) {
	// create a fake kube client
	fakeKubeClient := &clientgofake.Clientset{}
	// create a fake sc client
	fakeCatalogClient := &servicecatalogclientset.Clientset{}

	fakeOSBClient := fakeosb.NewFakeClient(config) // error should always be nil
	brokerClFunc := fakeosb.ReturnFakeClientFunc(fakeOSBClient)

	// create informers
	informerFactory := servicecataloginformers.NewSharedInformerFactory(fakeCatalogClient, 0)
	serviceCatalogSharedInformers := informerFactory.Servicecatalog().V1alpha1()

	fakeRecorder := record.NewFakeRecorder(5)

	// create a test controller
	testController, err := NewController(
		fakeKubeClient,
		fakeCatalogClient.ServicecatalogV1alpha1(),
		serviceCatalogSharedInformers.Brokers(),
		serviceCatalogSharedInformers.ServiceClasses(),
		serviceCatalogSharedInformers.Instances(),
		serviceCatalogSharedInformers.Bindings(),
		brokerClFunc,
		24*time.Hour,
		osb.Version2_12().HeaderValue(),
		fakeRecorder,
	)
	if err != nil {
		t.Fatal(err)
	}

	return fakeKubeClient, fakeCatalogClient, fakeOSBClient, testController.(*controller), serviceCatalogSharedInformers
}

type testControllerWithBrokerServer struct {
	FakeKubeClient      *clientgofake.Clientset
	FakeCatalogClient   *servicecatalogclientset.Clientset
	Controller          *controller
	Informers           v1alpha1informers.Interface
	BrokerServerHandler *fakebrokerserver.Handler
	BrokerServer        *httptest.Server
}

func (t *testControllerWithBrokerServer) Close() {
	t.BrokerServer.Close()
}

func newTestControllerWithBrokerServer(
	brokerUsername,
	brokerPassword string,
) (*testControllerWithBrokerServer, error) {
	// create a fake kube client
	fakeKubeClient := &clientgofake.Clientset{}
	// create a fake sc client
	fakeCatalogClient := &servicecatalogclientset.Clientset{}

	brokerHandler := fakebrokerserver.NewHandler()
	brokerServer := fakebrokerserver.Run(brokerHandler, brokerUsername, brokerPassword)

	// create informers
	informerFactory := servicecataloginformers.NewSharedInformerFactory(fakeCatalogClient, 0)
	serviceCatalogSharedInformers := informerFactory.Servicecatalog().V1alpha1()

	fakeRecorder := record.NewFakeRecorder(5)

	// create a test controller
	testController, err := NewController(
		fakeKubeClient,
		fakeCatalogClient.ServicecatalogV1alpha1(),
		serviceCatalogSharedInformers.Brokers(),
		serviceCatalogSharedInformers.ServiceClasses(),
		serviceCatalogSharedInformers.Instances(),
		serviceCatalogSharedInformers.Bindings(),
		osb.NewClient,
		24*time.Hour,
		osb.Version2_12().HeaderValue(),
		fakeRecorder,
	)
	if err != nil {
		return nil, err
	}

	return &testControllerWithBrokerServer{
		FakeKubeClient:      fakeKubeClient,
		FakeCatalogClient:   fakeCatalogClient,
		Controller:          testController.(*controller),
		Informers:           serviceCatalogSharedInformers,
		BrokerServerHandler: brokerHandler,
		BrokerServer:        brokerServer,
	}, nil
}

func getRecordedEvents(testController *controller) []string {
	source := testController.recorder.(*record.FakeRecorder).Events
	done := false
	events := []string{}
	for !done {
		select {
		case event := <-source:
			events = append(events, event)
		default:
			done = true
		}
	}
	return events
}

func assertNumEvents(t *testing.T, strings []string, number int) {
	if e, a := number, len(strings); e != a {
		fatalf(t, "Unexpected number of events: expected %v, got %v;\nevents: %+v", e, a, strings)
	}
}

// failfFunc is a type that defines the common signatures of T.Fatalf and
// T.Errorf.
type failfFunc func(t *testing.T, msg string, args ...interface{})

func fatalf(t *testing.T, msg string, args ...interface{}) {
	t.Log(string(debug.Stack()))
	t.Fatalf(msg, args...)
}

func errorf(t *testing.T, msg string, args ...interface{}) {
	t.Log(string(debug.Stack()))
	t.Errorf(msg, args...)
}

// assertion and expectation methods:
//
// - assertX will call t.Fatalf
// - expectX will call t.Errorf and return a boolean, allowing you to drive a 'continue'
//   in a table-type test

func assertNumberOfActions(t *testing.T, actions []clientgotesting.Action, number int) {
	testNumberOfActions(t, "" /* name */, fatalf, actions, number)
}

func expectNumberOfActions(t *testing.T, name string, actions []clientgotesting.Action, number int) bool {
	return testNumberOfActions(t, name, errorf, actions, number)
}

func testNumberOfActions(t *testing.T, name string, f failfFunc, actions []clientgotesting.Action, number int) bool {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	if e, a := number, len(actions); e != a {
		t.Logf("%+v\n", actions)
		f(t, "%vUnexpected number of actions: expected %v, got %v;\nactions: %+v", logContext, e, a, actions)
		return false
	}

	return true
}

func assertGet(t *testing.T, action clientgotesting.Action, obj interface{}) {
	assertActionFor(t, action, "get", "" /* subresource */, obj)
}

func assertCreate(t *testing.T, action clientgotesting.Action, obj interface{}) runtime.Object {
	return assertActionFor(t, action, "create", "" /* subresource */, obj)
}

func assertUpdate(t *testing.T, action clientgotesting.Action, obj interface{}) runtime.Object {
	return assertActionFor(t, action, "update", "" /* subresource */, obj)
}

func assertUpdateStatus(t *testing.T, action clientgotesting.Action, obj interface{}) runtime.Object {
	return assertActionFor(t, action, "update", "status", obj)
}

func expectUpdateStatus(t *testing.T, name string, action clientgotesting.Action, obj interface{}) (runtime.Object, bool) {
	return testActionFor(t, name, errorf, action, "update", "status", obj)
}

func assertDelete(t *testing.T, action clientgotesting.Action, obj interface{}) {
	assertActionFor(t, action, "delete", "" /* subresource */, obj)
}

func assertActionFor(t *testing.T, action clientgotesting.Action, verb, subresource string, obj interface{}) runtime.Object {
	r, _ := testActionFor(t, "" /* name */, fatalf, action, verb, subresource, obj)
	return r
}

func testActionFor(t *testing.T, name string, f failfFunc, action clientgotesting.Action, verb, subresource string, obj interface{}) (runtime.Object, bool) {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	if e, a := verb, action.GetVerb(); e != a {
		f(t, "%vUnexpected verb: expected %v, got %v", logContext, e, a)
		return nil, false
	}

	var resource string

	switch obj.(type) {
	case *v1alpha1.Broker:
		resource = "brokers"
	case *v1alpha1.ServiceClass:
		resource = "serviceclasses"
	case *v1alpha1.Instance:
		resource = "instances"
	case *v1alpha1.Binding:
		resource = "bindings"
	}

	if e, a := resource, action.GetResource().Resource; e != a {
		f(t, "%vUnexpected resource; expected %v, got %v", logContext, e, a)
		return nil, false
	}

	if e, a := subresource, action.GetSubresource(); e != a {
		f(t, "%vUnexpected subresource; expected %v, got %v", logContext, e, a)
		return nil, false
	}

	rtObject, ok := obj.(runtime.Object)
	if !ok {
		f(t, "%vObject %+v was not a runtime.Object", logContext, obj)
		return nil, false
	}

	paramAccessor, err := meta.Accessor(rtObject)
	if err != nil {
		f(t, "%vError creating ObjectMetaAccessor for param object %+v: %v", logContext, rtObject, err)
		return nil, false
	}

	var (
		objectMeta   metav1.Object
		fakeRtObject runtime.Object
	)

	switch verb {
	case "get":
		getAction, ok := action.(clientgotesting.GetAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to DeleteAction", logContext, action)
			return nil, false
		}

		if e, a := paramAccessor.GetName(), getAction.GetName(); e != a {
			f(t, "%vUnexpected name: expected %v, got %v", logContext, e, a)
			return nil, false
		}

		return nil, true
	case "delete":
		deleteAction, ok := action.(clientgotesting.DeleteAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to DeleteAction", logContext, action)
			return nil, false
		}

		if e, a := paramAccessor.GetName(), deleteAction.GetName(); e != a {
			f(t, "%vUnexpected name: expected %v, got %v", logContext, e, a)
			return nil, false
		}

		return nil, true
	case "create":
		createAction, ok := action.(clientgotesting.CreateAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to CreateAction", logContext, action)
			return nil, false
		}

		fakeRtObject = createAction.GetObject()
		objectMeta, err = meta.Accessor(fakeRtObject)
		if err != nil {
			f(t, "%vError creating ObjectMetaAccessor for %+v", logContext, fakeRtObject)
			return nil, false
		}
	case "update":
		updateAction, ok := action.(clientgotesting.UpdateAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to UpdateAction", logContext, action)
			return nil, false
		}

		fakeRtObject = updateAction.GetObject()
		objectMeta, err = meta.Accessor(fakeRtObject)
		if err != nil {
			f(t, "%vError creating ObjectMetaAccessor for %+v", logContext, fakeRtObject)
			return nil, false
		}
	}

	if e, a := paramAccessor.GetName(), objectMeta.GetName(); e != a {
		f(t, "%vUnexpected name: expected %v, got %v", logContext, e, a)
		return nil, false
	}

	fakeValue := reflect.ValueOf(fakeRtObject)
	paramValue := reflect.ValueOf(obj)

	if e, a := paramValue.Type(), fakeValue.Type(); e != a {
		f(t, "%vUnexpected type of object passed to fake client; expected %v, got %v", logContext, e, a)
		return nil, false
	}

	return fakeRtObject, true
}

func assertBrokerReadyTrue(t *testing.T, obj runtime.Object) {
	assertBrokerReadyCondition(t, obj, v1alpha1.ConditionTrue)
}

func assertBrokerReadyFalse(t *testing.T, obj runtime.Object) {
	assertBrokerReadyCondition(t, obj, v1alpha1.ConditionFalse)
}

func assertBrokerReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus) {
	broker, ok := obj.(*v1alpha1.Broker)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.Broker", obj)
	}

	for _, condition := range broker.Status.Conditions {
		if condition.Type == v1alpha1.BrokerConditionReady && condition.Status != status {
			fatalf(t, "ready condition had unexpected status; expected %v, got %v", status, condition.Status)
		}
	}
}

func assertInstanceReadyTrue(t *testing.T, obj runtime.Object) {
	assertInstanceReadyCondition(t, obj, v1alpha1.ConditionTrue)
}

func assertInstanceReadyFalse(t *testing.T, obj runtime.Object, reason ...string) {
	assertInstanceReadyCondition(t, obj, v1alpha1.ConditionFalse, reason...)
}

func assertInstanceReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus, reason ...string) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}

	for _, condition := range instance.Status.Conditions {
		if condition.Type == v1alpha1.InstanceConditionReady && condition.Status != status {
			fatalf(t, "ready condition had unexpected status; expected %v, got %v", status, condition.Status)
		}
		if len(reason) == 1 && condition.Reason != reason[0] {
			fatalf(t, "unexpected reason; expected %v, got %v", reason[0], condition.Reason)
		}
	}
}

func assertAsyncOpInProgressTrue(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if !instance.Status.AsyncOpInProgress {
		t.Fatalf("expected AsyncOpInProgress to be true but was %v", instance.Status.AsyncOpInProgress)
	}
}

func assertAsyncOpInProgressFalse(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if instance.Status.AsyncOpInProgress {
		t.Fatalf("expected AsyncOpInProgress to be false but was %v", instance.Status.AsyncOpInProgress)
	}
}

func assertInstanceLastOperation(t *testing.T, obj runtime.Object, operation string) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if instance.Status.LastOperation == nil {
		if operation != "" {
			t.Fatalf("Last Operation <nil> is not what was expected: %q", operation)
		}
	} else if *instance.Status.LastOperation != operation {
		t.Fatalf("Last Operation %q is not what was expected: %q", *instance.Status.LastOperation, operation)
	}
}

func assertInstanceDashboardURL(t *testing.T, obj runtime.Object, dashboardURL string) {
	instance, ok := obj.(*v1alpha1.Instance)
	if !ok {
		t.Fatalf("Couldn't convert object %+v into a *v1alpha1.Instance", obj)
	}
	if instance.Status.DashboardURL == nil {
		t.Fatal("DashboardURL was nil")
	} else if *instance.Status.DashboardURL != dashboardURL {
		t.Fatalf("Unexpected DashboardURL: expected %q, got %q", dashboardURL, *instance.Status.DashboardURL)
	}
}

func assertBindingReadyTrue(t *testing.T, obj runtime.Object) {
	assertBindingReadyCondition(t, obj, v1alpha1.ConditionTrue)
}

func assertBindingReadyFalse(t *testing.T, obj runtime.Object, reason ...string) {
	assertBindingReadyCondition(t, obj, v1alpha1.ConditionFalse, reason...)
}

func assertBindingReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus, reason ...string) {
	binding, ok := obj.(*v1alpha1.Binding)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.Binding", obj)
	}

	for _, condition := range binding.Status.Conditions {
		if condition.Type == v1alpha1.BindingConditionReady && condition.Status != status {
			t.Logf("ready condition: %+v", condition)
			fatalf(t, "ready condition had unexpected status; expected %v, got %v", status, condition.Status)
		}
		if len(reason) == 1 && condition.Reason != reason[0] {
			fatalf(t, "unexpected reason; expected %v, got %v", reason[0], condition.Reason)
		}
	}
}

func assertEmptyFinalizers(t *testing.T, obj runtime.Object) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		fatalf(t, "Error creating ObjectMetaAccessor for param object %+v: %v", obj, err)
	}

	if len(accessor.GetFinalizers()) != 0 {
		fatalf(t, "Unexpected number of finalizers; expected 0, got %v", len(accessor.GetFinalizers()))
	}
}

func assertNumberOfBrokerActions(t *testing.T, actions []fakeosb.Action, number int) {
	testNumberOfBrokerActions(t, "" /* name */, fatalf, actions, number)
}

func expectNumberOfBrokerActions(t *testing.T, name string, actions []fakeosb.Action, number int) bool {
	return testNumberOfBrokerActions(t, name, errorf, actions, number)
}

func testNumberOfBrokerActions(t *testing.T, name string, f failfFunc, actions []fakeosb.Action, number int) bool {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	if e, a := number, len(actions); e != a {
		t.Logf("%+v\n", actions)
		f(t, "%vUnexpected number of actions: expected %v, got %v", logContext, e, a)
		return false
	}

	return true
}

func noFakeActions() fakeosb.FakeClientConfiguration {
	return fakeosb.FakeClientConfiguration{}
}

func assertGetCatalog(t *testing.T, action fakeosb.Action) {
	if e, a := fakeosb.GetCatalog, action.Type; e != a {
		fatalf(t, "unexpected action type; expected %v, got %v", e, a)
	}
}

func assertProvision(t *testing.T, action fakeosb.Action, request *osb.ProvisionRequest) {
	if e, a := fakeosb.ProvisionInstance, action.Type; e != a {
		fatalf(t, "unexpected action type; expected %v, got %v", e, a)
	}

	if e, a := request, action.Request; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in provision request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
	}
}

func assertDeprovision(t *testing.T, action fakeosb.Action, request *osb.DeprovisionRequest) {
	if e, a := fakeosb.DeprovisionInstance, action.Type; e != a {
		fatalf(t, "unexpected action type; expected %v, got %v", e, a)
	}

	if e, a := request, action.Request; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in deprovision request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
	}
}

func assertPollLastOperation(t *testing.T, action fakeosb.Action, request *osb.LastOperationRequest) {
	if e, a := fakeosb.PollLastOperation, action.Type; e != a {
		fatalf(t, "unexpected action type; expected %v, got %v", e, a)
	}

	if e, a := request, action.Request; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in last operation request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
	}
}

func assertBind(t *testing.T, action fakeosb.Action, request *osb.BindRequest) {
	if e, a := fakeosb.Bind, action.Type; e != a {
		fatalf(t, "unexpected action type; expected %v, got %v", e, a)
	}

	if e, a := request, action.Request; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in bind request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
	}
}

func assertUnbind(t *testing.T, action fakeosb.Action, request *osb.UnbindRequest) {
	if e, a := fakeosb.Unbind, action.Type; e != a {
		fatalf(t, "unexpected action type; expected %v, got %v", e, a)
	}

	if e, a := request, action.Request; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in bind request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
	}
}

func getTestCatalogConfig() fakeosb.FakeClientConfiguration {
	return fakeosb.FakeClientConfiguration{
		CatalogReaction: &fakeosb.CatalogReaction{
			Response: getTestCatalog(),
		},
	}
}

func addGetNamespaceReaction(fakeKubeClient *clientgofake.Clientset) {
	fakeKubeClient.AddReactor("get", "namespaces", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(testNsUID),
			},
		}, nil
	})
}

func addGetSecretNotFoundReaction(fakeKubeClient *clientgofake.Clientset) {
	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), action.(clientgotesting.GetAction).GetName())
	})
}

func addGetSecretReaction(fakeKubeClient *clientgofake.Clientset, secret *v1.Secret) {
	fakeKubeClient.AddReactor("get", "secrets", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		return true, secret, nil
	})
}
