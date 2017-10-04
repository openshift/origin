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
	"fmt"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	"github.com/ghodss/yaml"
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

	"github.com/kubernetes-incubator/service-catalog/test/fake"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/sets"
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
// Other controller_*_test.go files contain tests related to the reconciliation
// loops for the different catalog API resources.

const (
	serviceClassGUID            = "SCGUID"
	planGUID                    = "PGUID"
	nonbindableServiceClassGUID = "UNBINDABLE-SERVICE"
	nonbindablePlanGUID         = "UNBINDABLE-PLAN"
	instanceGUID                = "IGUID"
	bindingGUID                 = "BGUID"

	testServiceBrokerName                   = "test-broker"
	testServiceClassName                    = "test-serviceclass"
	testNonbindableServiceClassName         = "test-unbindable-serviceclass"
	testServicePlanName                     = "test-plan"
	testNonbindableServicePlanName          = "test-unbindable-plan"
	testServiceInstanceName                 = "test-instance"
	testServiceInstanceCredentialName       = "test-binding"
	testNamespace                           = "test-ns"
	testServiceInstanceCredentialSecretName = "test-secret"
	testOperation                           = "test-operation"
	testNsUID                               = "test-ns-uid"
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
      "displayName": "The Fake ServiceBroker"
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

type testTimeoutError struct{}

func (e testTimeoutError) Error() string {
	return "timed out"
}

func (e testTimeoutError) Timeout() bool {
	return true
}

func getTestTimeoutError() error {
	return testTimeoutError{}
}

type originatingIdentityTestCase struct {
	name                        string
	includeUserInfo             bool
	enableOriginatingIdentity   bool
	expectedOriginatingIdentity bool
}

var originatingIdentityTestCases = []originatingIdentityTestCase{
	{
		name:                        "originating identity not included when feature disabled",
		includeUserInfo:             true,
		enableOriginatingIdentity:   false,
		expectedOriginatingIdentity: false,
	},
	{
		name:                        "originating identity not included when no creating user info",
		includeUserInfo:             false,
		enableOriginatingIdentity:   true,
		expectedOriginatingIdentity: false,
	},
	{
		name:                        "originating identity included",
		includeUserInfo:             true,
		enableOriginatingIdentity:   true,
		expectedOriginatingIdentity: true,
	},
}

var testUserInfo = &v1alpha1.UserInfo{
	Username: "fakeusername",
	UID:      "fakeuid",
	Groups:   []string{"fakegroup1"},
	Extra: map[string]v1alpha1.ExtraValue{
		"fakekey": v1alpha1.ExtraValue([]string{"fakevalue"}),
	},
}

const testOriginatingIdentityValue = `{
	"username": "fakeusername",
	"uid": "fakeuid",
	"groups": ["fakegroup1"],
	"fakekey": ["fakevalue"]
}`

var testOriginatingIdentity = &osb.AlphaOriginatingIdentity{
	Platform: originatingIdentityPlatform,
	Value:    testOriginatingIdentityValue,
}

// broker used in most of the tests that need a broker
func getTestServiceBroker() *v1alpha1.ServiceBroker {
	return &v1alpha1.ServiceBroker{
		ObjectMeta: metav1.ObjectMeta{Name: testServiceBrokerName},
		Spec: v1alpha1.ServiceBrokerSpec{
			URL:            "https://example.com",
			RelistBehavior: v1alpha1.ServiceBrokerRelistBehaviorDuration,
			RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
		},
	}
}

func getTestServiceBrokerWithStatus(status v1alpha1.ConditionStatus) *v1alpha1.ServiceBroker {
	broker := getTestServiceBroker()
	broker.Status = v1alpha1.ServiceBrokerStatus{
		Conditions: []v1alpha1.ServiceBrokerCondition{{
			Type:               v1alpha1.ServiceBrokerConditionReady,
			Status:             status,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
	}

	return broker
}

func getTestServiceBrokerWithAuth(authInfo *v1alpha1.ServiceBrokerAuthInfo) *v1alpha1.ServiceBroker {
	broker := getTestServiceBroker()
	broker.Spec.AuthInfo = authInfo
	return broker
}

// a bindable service class wired to the result of getTestServiceBroker()
func getTestServiceClass() *v1alpha1.ServiceClass {
	return &v1alpha1.ServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: serviceClassGUID},
		Spec: v1alpha1.ServiceClassSpec{
			ServiceBrokerName: testServiceBrokerName,
			Description:       "a test service",
			ExternalName:      testServiceClassName,
			ExternalID:        serviceClassGUID,
			Bindable:          true,
		},
	}
}

func getTestServicePlan() *v1alpha1.ServicePlan {
	return &v1alpha1.ServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: planGUID},
		Spec: v1alpha1.ServicePlanSpec{
			ServiceBrokerName: testServiceBrokerName,
			ExternalID:        planGUID,
			ExternalName:      testServicePlanName,
			Bindable:          truePtr(),
			ServiceClassRef: v1.LocalObjectReference{
				Name: serviceClassGUID,
			},
		},
	}
}

func getTestServicePlanNonbindable() *v1alpha1.ServicePlan {
	return &v1alpha1.ServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: nonbindablePlanGUID},
		Spec: v1alpha1.ServicePlanSpec{
			ExternalName: testNonbindableServicePlanName,
			ExternalID:   nonbindablePlanGUID,
			Bindable:     falsePtr(),
			ServiceClassRef: v1.LocalObjectReference{
				Name: serviceClassGUID,
			},
		},
	}
}

// an unbindable service class wired to the result of getTestServiceBroker()
func getTestNonbindableServiceClass() *v1alpha1.ServiceClass {
	return &v1alpha1.ServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: nonbindableServiceClassGUID},
		Spec: v1alpha1.ServiceClassSpec{
			ServiceBrokerName: testServiceBrokerName,
			ExternalName:      testNonbindableServiceClassName,
			ExternalID:        nonbindableServiceClassGUID,
			Bindable:          false,
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
						Name:        testServicePlanName,
						Free:        truePtr(),
						ID:          planGUID,
						Description: "a test plan",
					},
					{
						Name:        testNonbindableServicePlanName,
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
// and getTestServicePlan()
// This version sets:
// ExternalServiceClassName and ExternalServicePlanName as well
// as ServiceClassRef and ServicePlanRef which means that the
// ServiceClass and ServicePlan are fetched using
// Service[Class|Plan]Lister.get(spec.Service[Class|Plan]Ref.Name)
func getTestServiceInstanceWithRefs() *v1alpha1.ServiceInstance {
	sc := getTestServiceInstance()
	sc.Spec.ServiceClassRef = &v1.ObjectReference{Name: serviceClassGUID}
	sc.Spec.ServicePlanRef = &v1.ObjectReference{Name: planGUID}
	return sc
}

// instance referencing the result of getTestServiceClass()
// and getTestServicePlan()
// This version sets:
// ExternalServiceClassName and ExternalServicePlanName, so depending on the
// test, you may need to add reactors that deal with List due to the need
// to resolve Names to IDs for both ServiceClass and ServicePlan
func getTestServiceInstance() *v1alpha1.ServiceInstance {
	return &v1alpha1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1alpha1.ServiceInstanceSpec{
			ExternalServiceClassName: testServiceClassName,
			ExternalServicePlanName:  testServicePlanName,
			ExternalID:               instanceGUID,
		},
	}
}

// an instance referencing the result of getTestNonbindableServiceClass, on the non-bindable plan.
func getTestNonbindableServiceInstance() *v1alpha1.ServiceInstance {
	i := getTestServiceInstance()
	i.Spec.ExternalServiceClassName = testNonbindableServiceClassName
	i.Spec.ExternalServicePlanName = testNonbindableServicePlanName
	i.Spec.ServiceClassRef = &v1.ObjectReference{Name: nonbindableServiceClassGUID}
	i.Spec.ServicePlanRef = &v1.ObjectReference{Name: nonbindablePlanGUID}

	return i
}

// an instance referencing the result of getTestNonbindableServiceClass, on the bindable plan.
func getTestServiceInstanceNonbindableServiceBindablePlan() *v1alpha1.ServiceInstance {
	i := getTestNonbindableServiceInstance()
	i.Spec.ExternalServicePlanName = testServicePlanName
	i.Spec.ServicePlanRef = &v1.ObjectReference{Name: planGUID}

	return i
}

func getTestServiceInstanceBindableServiceNonbindablePlan() *v1alpha1.ServiceInstance {
	i := getTestServiceInstanceWithRefs()
	i.Spec.ExternalServicePlanName = testNonbindableServicePlanName
	i.Spec.ServicePlanRef = &v1.ObjectReference{Name: nonbindablePlanGUID}

	return i
}

func getTestServiceInstanceWithStatus(status v1alpha1.ConditionStatus) *v1alpha1.ServiceInstance {
	instance := getTestServiceInstanceWithRefs()
	instance.Status = v1alpha1.ServiceInstanceStatus{
		Conditions: []v1alpha1.ServiceInstanceCondition{{
			Type:               v1alpha1.ServiceInstanceConditionReady,
			Status:             status,
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
	}

	return instance
}

func getTestServiceInstanceWithFailedStatus() *v1alpha1.ServiceInstance {
	instance := getTestServiceInstanceWithRefs()
	instance.Status = v1alpha1.ServiceInstanceStatus{
		Conditions: []v1alpha1.ServiceInstanceCondition{{
			Type:   v1alpha1.ServiceInstanceConditionFailed,
			Status: v1alpha1.ConditionTrue,
		}},
	}

	return instance
}

// getTestServiceInstanceAsync returns an instance in async mode
func getTestServiceInstanceAsyncProvisioning(operation string) *v1alpha1.ServiceInstance {
	instance := getTestServiceInstanceWithRefs()
	if operation != "" {
		instance.Status.LastOperation = &operation
	}

	operationStartTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	instance.Status = v1alpha1.ServiceInstanceStatus{
		Conditions: []v1alpha1.ServiceInstanceCondition{{
			Type:               v1alpha1.ServiceInstanceConditionReady,
			Status:             v1alpha1.ConditionFalse,
			Message:            "Provisioning",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
		AsyncOpInProgress:  true,
		OperationStartTime: &operationStartTime,
		CurrentOperation:   v1alpha1.ServiceInstanceOperationProvision,
		InProgressProperties: &v1alpha1.ServiceInstancePropertiesState{
			ExternalServicePlanName: testServicePlanName,
		},
	}
	if operation != "" {
		instance.Status.LastOperation = &operation
	}

	return instance
}

func getTestServiceInstanceAsyncDeprovisioning(operation string) *v1alpha1.ServiceInstance {
	instance := getTestServiceInstanceWithRefs()
	instance.Generation = 2
	if operation != "" {
		instance.Status.LastOperation = &operation
	}

	operationStartTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))
	instance.Status = v1alpha1.ServiceInstanceStatus{
		Conditions: []v1alpha1.ServiceInstanceCondition{{
			Type:               v1alpha1.ServiceInstanceConditionReady,
			Status:             v1alpha1.ConditionFalse,
			Message:            "Deprovisioning",
			LastTransitionTime: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		}},
		AsyncOpInProgress:    true,
		OperationStartTime:   &operationStartTime,
		CurrentOperation:     v1alpha1.ServiceInstanceOperationDeprovision,
		ReconciledGeneration: 1,
		ExternalProperties: &v1alpha1.ServiceInstancePropertiesState{
			ExternalServicePlanName: testServicePlanName,
		},
	}
	if operation != "" {
		instance.Status.LastOperation = &operation
	}

	// Set the deleted timestamp to simulate deletion
	ts := metav1.NewTime(time.Now().Add(-5 * time.Minute))
	instance.DeletionTimestamp = &ts
	return instance
}

func getTestServiceInstanceAsyncDeprovisioningWithFinalizer(operation string) *v1alpha1.ServiceInstance {
	instance := getTestServiceInstanceAsyncDeprovisioning(operation)
	instance.ObjectMeta.Finalizers = []string{v1alpha1.FinalizerServiceCatalog}
	return instance
}

// binding referencing the result of getTestServiceInstance()
func getTestServiceInstanceCredential() *v1alpha1.ServiceInstanceCredential {
	return &v1alpha1.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:       testServiceInstanceCredentialName,
			Namespace:  testNamespace,
			Generation: 1,
		},
		Spec: v1alpha1.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{Name: testServiceInstanceName},
			ExternalID:         bindingGUID,
		},
	}
}

func getTestServiceInstanceCredentialWithFailedStatus() *v1alpha1.ServiceInstanceCredential {
	binding := getTestServiceInstanceCredential()
	binding.Status = v1alpha1.ServiceInstanceCredentialStatus{
		Conditions: []v1alpha1.ServiceInstanceCredentialCondition{{
			Type:   v1alpha1.ServiceInstanceCredentialConditionFailed,
			Status: v1alpha1.ConditionTrue,
		}},
	}

	return binding
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
	serviceClasses, servicePlans, err := convertCatalog(&osb.CatalogResponse{})
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 0 {
		t.Fatalf("Expected 0 serviceclasses for empty catalog, but got: %d", len(serviceClasses))
	}
	if len(servicePlans) != 0 {
		t.Fatalf("Expected 0 serviceplans for empty catalog, but got: %d", len(servicePlans))
	}
}

func TestCatalogConversion(t *testing.T) {
	catalog := &osb.CatalogResponse{}
	err := json.Unmarshal([]byte(testCatalog), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}
	serviceClasses, servicePlans, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 1 {
		t.Fatalf("Expected 1 serviceclasses for testCatalog, but got: %d", len(serviceClasses))
	}
	if len(servicePlans) != 2 {
		t.Fatalf("Expected 2 plans for testCatalog, but got: %d", len(servicePlans))
	}

	checkPlan(servicePlans[0], "d3031751-XXXX-XXXX-XXXX-a42377d3320e", "fake-plan-1", "Shared fake Server, 5tb persistent disk, 40 max concurrent connections", t)
	checkPlan(servicePlans[1], "0f4008b5-XXXX-XXXX-XXXX-dace631cd648", "fake-plan-2", "Shared fake Server, 5tb persistent disk, 40 max concurrent connections. 100 async", t)
}

func TestCatalogConversionWithAlphaParameterSchemas(t *testing.T) {
	catalog := &osb.CatalogResponse{}
	err := json.Unmarshal([]byte(alphaParameterSchemaCatalogBytes), &catalog)
	if err != nil {
		t.Fatalf("Failed to unmarshal test catalog: %v", err)
	}
	serviceClasses, servicePlans, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}
	if len(serviceClasses) != 1 {
		t.Fatalf("Expected 1 serviceclasses for testCatalog, but got: %d", len(serviceClasses))
	}
	if len(servicePlans) != 1 {
		t.Fatalf("Expected 1 plan for testCatalog, but got: %d", len(servicePlans))
	}

	plan := servicePlans[0]
	if plan.Spec.ServiceInstanceCreateParameterSchema == nil {
		t.Fatalf("Expected plan.ServiceInstanceCreateParameterSchema to be set, but was nil")
	}

	cSchema := make(map[string]interface{})
	if err := json.Unmarshal(plan.Spec.ServiceInstanceCreateParameterSchema.Raw, &cSchema); err == nil {
		schema := make(map[string]interface{})
		if err := json.Unmarshal([]byte(instanceParameterSchemaBytes), &schema); err != nil {
			t.Fatalf("Error unmarshalling schema bytes: %v", err)
		}

		if e, a := schema, cSchema; !reflect.DeepEqual(e, a) {
			t.Fatalf("Unexpected value of alphaInstanceCreateParameterSchema; expected %v, got %v", e, a)
		}
	}

	if plan.Spec.ServiceInstanceUpdateParameterSchema == nil {
		t.Fatalf("Expected plan.ServiceInstanceUpdateParameterSchema to be set, but was nil")
	}
	m := make(map[string]string)
	if err := json.Unmarshal(plan.Spec.ServiceInstanceUpdateParameterSchema.Raw, &m); err == nil {
		if e, a := "zap", m["baz"]; e != a {
			t.Fatalf("Unexpected value of alphaInstanceUpdateParameterSchema; expected %v, got %v", e, a)
		}
	}

	if plan.Spec.ServiceInstanceCredentialCreateParameterSchema == nil {
		t.Fatalf("Expected plan.ServiceInstanceCredentialCreateParameterSchema to be set, but was nil")
	}
	m = make(map[string]string)
	if err := json.Unmarshal(plan.Spec.ServiceInstanceCredentialCreateParameterSchema.Raw, &m); err == nil {
		if e, a := "blu", m["zoo"]; e != a {
			t.Fatalf("Unexpected value of alphaServiceInstanceCredentialCreateParameterSchema; expected %v, got %v", e, a)
		}
	}
}

func checkPlan(plan *v1alpha1.ServicePlan, planID, planName, planDescription string, t *testing.T) {
	if plan.Name != planID {
		t.Errorf("Expected plan name to be %q, but was: %q", planID, plan.Name)
	}
	if plan.Spec.ExternalID != planID {
		t.Errorf("Expected plan ExternalID to be %q, but was: %q", planID, plan.Spec.ExternalID)
	}
	if plan.Spec.ExternalName != planName {
		t.Errorf("Expected plan ExternalName to be %q, but was: %q", planName, plan.Spec.ExternalName)
	}
	if plan.Spec.Description != planDescription {
		t.Errorf("Expected plan description to be %q, but was: %q", planDescription, plan.Spec.Description)
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
      "id": "bindable-id",
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
      "id": "unbindable-id",
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

	aclasses, aplans, err := convertCatalog(catalog)
	if err != nil {
		t.Fatalf("Failed to convertCatalog: %v", err)
	}

	eclasses := []*v1alpha1.ServiceClass{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bindable-id",
			},
			Spec: v1alpha1.ServiceClassSpec{
				ExternalName: "bindable",
				ExternalID:   "bindable-id",
				Bindable:     true,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "unbindable-id",
			},
			Spec: v1alpha1.ServiceClassSpec{
				ExternalName: "unbindable",
				ExternalID:   "unbindable-id",
				Bindable:     false,
			},
		},
	}

	eplans := []*v1alpha1.ServicePlan{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s1_plan1_id",
			},
			Spec: v1alpha1.ServicePlanSpec{
				ExternalID:   "s1_plan1_id",
				ExternalName: "bindable-bindable",
				Bindable:     nil,
				ServiceClassRef: v1.LocalObjectReference{
					Name: "bindable-id",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s1_plan2_id",
			},
			Spec: v1alpha1.ServicePlanSpec{
				ExternalName: "bindable-unbindable",
				ExternalID:   "s1_plan2_id",
				Bindable:     falsePtr(),
				ServiceClassRef: v1.LocalObjectReference{
					Name: "bindable-id",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s2_plan1_id",
			},
			Spec: v1alpha1.ServicePlanSpec{
				ExternalName: "unbindable-unbindable",
				ExternalID:   "s2_plan1_id",
				Bindable:     nil,
				ServiceClassRef: v1.LocalObjectReference{
					Name: "unbindable-id",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "s2_plan2_id",
			},
			Spec: v1alpha1.ServicePlanSpec{
				ExternalName: "unbindable-bindable",
				ExternalID:   "s2_plan2_id",
				Bindable:     truePtr(),
				ServiceClassRef: v1.LocalObjectReference{
					Name: "unbindable-id",
				},
			},
		},
	}

	if !reflect.DeepEqual(eclasses, aclasses) {
		t.Errorf("Unexpected diff between expected and actual serviceclasses: %v", diff.ObjectReflectDiff(eclasses, aclasses))
	}
	if !reflect.DeepEqual(eplans, aplans) {
		t.Errorf("Unexpected diff between expected and actual serviceplans: %v", diff.ObjectReflectDiff(eplans, aplans))
	}

}

func TestIsServiceBrokerReady(t *testing.T) {
	cases := []struct {
		name  string
		input *v1alpha1.ServiceInstance
		ready bool
	}{
		{
			name:  "ready",
			input: getTestServiceInstanceWithStatus(v1alpha1.ConditionTrue),
			ready: true,
		},
		{
			name:  "no status",
			input: getTestServiceInstance(),
			ready: false,
		},
		{
			name:  "not ready",
			input: getTestServiceInstanceWithStatus(v1alpha1.ConditionFalse),
			ready: false,
		},
	}

	for _, tc := range cases {
		if e, a := tc.ready, isServiceInstanceReady(tc.input); e != a {
			t.Errorf("%v: expected result %v, got %v", tc.name, e, a)
		}
	}
}

func TestIsPlanBindable(t *testing.T) {
	serviceClass := func(bindable bool) *v1alpha1.ServiceClass {
		serviceClass := getTestServiceClass()
		serviceClass.Spec.Bindable = bindable
		return serviceClass
	}

	servicePlan := func(bindable *bool) *v1alpha1.ServicePlan {
		return &v1alpha1.ServicePlan{
			Spec: v1alpha1.ServicePlanSpec{
				Bindable: bindable,
			},
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
	*fake.Clientset,
	*fakeosb.FakeClient,
	*controller,
	v1alpha1informers.Interface) {
	// create a fake kube client
	fakeKubeClient := &clientgofake.Clientset{}
	// create a fake sc client
	fakeCatalogClient := &fake.Clientset{&servicecatalogclientset.Clientset{}}

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
		serviceCatalogSharedInformers.ServiceBrokers(),
		serviceCatalogSharedInformers.ServiceClasses(),
		serviceCatalogSharedInformers.ServiceInstances(),
		serviceCatalogSharedInformers.ServiceInstanceCredentials(),
		serviceCatalogSharedInformers.ServicePlans(),
		brokerClFunc,
		24*time.Hour,
		osb.LatestAPIVersion().HeaderValue(),
		fakeRecorder,
		7*24*time.Hour,
	)
	if err != nil {
		t.Fatal(err)
	}

	return fakeKubeClient, fakeCatalogClient, fakeOSBClient, testController.(*controller), serviceCatalogSharedInformers
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

func assertList(t *testing.T, action clientgotesting.Action, obj interface{}, listRestrictions clientgotesting.ListRestrictions) {
	assertActionFor(t, action, "list", "" /* subresource */, obj)
	// Cast is ok since in the method above it's checked to be ListAction
	assertListRestrictions(t, listRestrictions, action.(clientgotesting.ListAction).GetListRestrictions())
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

func assertUpdateReference(t *testing.T, action clientgotesting.Action, obj interface{}) runtime.Object {
	return assertActionFor(t, action, "update", "reference", obj)
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
		f(t, "%vUnexpected verb: expected %v, got %v\n\tactual action %q", logContext, e, a, action)
		return nil, false
	}

	var resource string

	switch obj.(type) {
	case *v1alpha1.ServiceBroker:
		resource = "servicebrokers"
	case *v1alpha1.ServiceClass:
		resource = "serviceclasses"
	case *v1alpha1.ServicePlan:
		resource = "serviceplans"
	case *v1alpha1.ServiceInstance:
		resource = "serviceinstances"
	case *v1alpha1.ServiceInstanceCredential:
		resource = "serviceinstancecredentials"
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
	case "list":
		_, ok := action.(clientgotesting.ListAction)
		if !ok {
			f(t, "%vUnexpected type; failed to convert action %+v to ListAction", logContext, action)
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

func assertServiceBrokerReadyTrue(t *testing.T, obj runtime.Object) {
	assertServiceBrokerCondition(t, obj, v1alpha1.ServiceBrokerConditionReady, v1alpha1.ConditionTrue)
}

func assertServiceBrokerReadyFalse(t *testing.T, obj runtime.Object) {
	assertServiceBrokerCondition(t, obj, v1alpha1.ServiceBrokerConditionReady, v1alpha1.ConditionFalse)
}

func assertServiceBrokerCondition(t *testing.T, obj runtime.Object, conditionType v1alpha1.ServiceBrokerConditionType, status v1alpha1.ConditionStatus) {
	broker, ok := obj.(*v1alpha1.ServiceBroker)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceBroker", obj)
	}

	for _, condition := range broker.Status.Conditions {
		if condition.Type == conditionType && condition.Status != status {
			fatalf(t, "%v condition had unexpected status; expected %v, got %v", conditionType, status, condition.Status)
		}
	}
}

func assertServiceBrokerOperationStartTimeSet(t *testing.T, obj runtime.Object, isOperationStartTimeSet bool) {
	broker, ok := obj.(*v1alpha1.ServiceBroker)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceBroker", obj)
	}

	if e, a := isOperationStartTimeSet, broker.Status.OperationStartTime != nil; e != a {
		if e {
			fatalf(t, "expected OperationStartTime to not be nil, but was")
		} else {
			fatalf(t, "expected OperationStartTime to be nil, but was not")
		}
	}
}

func assertServiceInstanceReadyTrue(t *testing.T, obj runtime.Object, reason ...string) {
	assertServiceInstanceReadyCondition(t, obj, v1alpha1.ConditionTrue, reason...)
}

func assertServiceInstanceReadyFalse(t *testing.T, obj runtime.Object, reason ...string) {
	assertServiceInstanceReadyCondition(t, obj, v1alpha1.ConditionFalse, reason...)
}

func assertServiceInstanceReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus, reason ...string) {
	assertServiceInstanceCondition(t, obj, v1alpha1.ServiceInstanceConditionReady, status, reason...)
}

func assertServiceInstanceCondition(t *testing.T, obj runtime.Object, conditionType v1alpha1.ServiceInstanceConditionType, status v1alpha1.ConditionStatus, reason ...string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	foundCondition := false
	for _, condition := range instance.Status.Conditions {
		if condition.Type == conditionType {
			foundCondition = true
			if condition.Status != status {
				fatalf(t, "%v condition had unexpected status; expected %v, got %v", conditionType, status, condition.Status)
			}
			if len(reason) == 1 && condition.Reason != reason[0] {
				fatalf(t, "unexpected reason; expected %v, got %v", reason[0], condition.Reason)
			}
		}
	}

	if !foundCondition {
		fatalf(t, "%v condition not found", conditionType)
	}
}

func assertServiceInstanceConditionsCount(t *testing.T, obj runtime.Object, count int) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	if e, a := count, len(instance.Status.Conditions); e != a {
		t.Fatalf("Expected %v condition, got %v", e, a)
	}
}

func assertServiceInstanceCurrentOperationClear(t *testing.T, obj runtime.Object) {
	assertServiceInstanceCurrentOperation(t, obj, "")
}

func assertServiceInstanceCurrentOperation(t *testing.T, obj runtime.Object, currentOperation v1alpha1.ServiceInstanceOperation) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	if e, a := currentOperation, instance.Status.CurrentOperation; e != a {
		fatalf(t, "unexpected current operation: expected %q, got %q", e, a)
	}
}

func assertServiceInstanceReconciledGeneration(t *testing.T, obj runtime.Object, reconciledGeneration int64) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	if e, a := reconciledGeneration, instance.Status.ReconciledGeneration; e != a {
		fatalf(t, "unexpected reconciled generation: expected %v, got %v", e, a)
	}
}

func assertServiceInstanceOperationStartTimeSet(t *testing.T, obj runtime.Object, isOperationStartTimeSet bool) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	if e, a := isOperationStartTimeSet, instance.Status.OperationStartTime != nil; e != a {
		if e {
			fatalf(t, "expected OperationStartTime to not be nil, but was")
		} else {
			fatalf(t, "expected OperationStartTime to be nil, but was not")
		}
	}
}

func assertAsyncOpInProgressTrue(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	if !instance.Status.AsyncOpInProgress {
		fatalf(t, "expected AsyncOpInProgress to be true but was %v", instance.Status.AsyncOpInProgress)
	}
}

func assertAsyncOpInProgressFalse(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	if instance.Status.AsyncOpInProgress {
		fatalf(t, "expected AsyncOpInProgress to be false but was %v", instance.Status.AsyncOpInProgress)
	}
}

func assertServiceInstanceOrphanMitigationInProgressTrue(t *testing.T, obj runtime.Object) {
	testServiceInstanceOrphanMitigationInProgress(t, "" /* name */, fatalf, obj, true)
}

func assertServiceInstanceOrphanMitigationInProgressFalse(t *testing.T, obj runtime.Object) {
	testServiceInstanceOrphanMitigationInProgress(t, "" /* name */, fatalf, obj, false)
}

func expectServiceInstanceOrphanMitigationInProgressTrue(t *testing.T, name string, obj runtime.Object) bool {
	return testServiceInstanceOrphanMitigationInProgress(t, name, fatalf, obj, true)
}

func expectServiceInstanceOrphanMitigationInProgressFalse(t *testing.T, name string, obj runtime.Object) bool {
	return testServiceInstanceOrphanMitigationInProgress(t, name, fatalf, obj, false)
}

func testServiceInstanceOrphanMitigationInProgress(t *testing.T, name string, f failfFunc, obj runtime.Object, expected bool) bool {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		f(t, "%vCouldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	actual := instance.Status.OrphanMitigationInProgress
	if actual != expected {
		f(t, "%vexpected OrphanMitigationInProgress to be %v but was %v", logContext, expected, actual)
		return false
	}

	return true
}

func assertServiceInstanceLastOperation(t *testing.T, obj runtime.Object, operation string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	if instance.Status.LastOperation == nil {
		if operation != "" {
			fatalf(t, "Last Operation <nil> is not what was expected: %q", operation)
		}
	} else if *instance.Status.LastOperation != operation {
		fatalf(t, "Last Operation %q is not what was expected: %q", *instance.Status.LastOperation, operation)
	}
}

func assertServiceInstanceDashboardURL(t *testing.T, obj runtime.Object, dashboardURL string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	if instance.Status.DashboardURL == nil {
		fatalf(t, "DashboardURL was nil")
	} else if *instance.Status.DashboardURL != dashboardURL {
		fatalf(t, "Unexpected DashboardURL: expected %q, got %q", dashboardURL, *instance.Status.DashboardURL)
	}
}

func assertServiceInstanceErrorBeforeRequest(t *testing.T, obj runtime.Object, reason string, originalInstance *v1alpha1.ServiceInstance) {
	assertServiceInstanceReadyFalse(t, obj, reason)
	assertServiceInstanceCurrentOperationClear(t, obj)
	assertServiceInstanceOperationStartTimeSet(t, obj, false)
	assertServiceInstanceReconciledGeneration(t, obj, originalInstance.Status.ReconciledGeneration)
	assertAsyncOpInProgressFalse(t, obj)
	assertServiceInstanceOrphanMitigationInProgressFalse(t, obj)
	assertServiceInstanceInProgressPropertiesNil(t, obj)
	assertServiceInstanceExternalPropertiesUnchanged(t, obj, originalInstance)
}

func assertServiceInstanceOperationInProgress(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, planName string, originalInstance *v1alpha1.ServiceInstance) {
	assertServiceInstanceOperationInProgressWithParameters(t, obj, operation, planName, nil, "", originalInstance)
}

func assertServiceInstanceOperationInProgressWithParameters(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, planName string, inProgressParameters map[string]interface{}, inProgressParametersChecksum string, originalInstance *v1alpha1.ServiceInstance) {
	reason := ""
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		reason = provisioningInFlightReason
	case v1alpha1.ServiceInstanceOperationDeprovision:
		reason = deprovisioningInFlightReason
	}
	assertServiceInstanceReadyFalse(t, obj, reason)
	assertServiceInstanceCurrentOperation(t, obj, operation)
	assertServiceInstanceOperationStartTimeSet(t, obj, true)
	assertServiceInstanceReconciledGeneration(t, obj, originalInstance.Status.ReconciledGeneration)
	assertAsyncOpInProgressFalse(t, obj)
	assertServiceInstanceOrphanMitigationInProgressFalse(t, obj)
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		assertServiceInstanceInProgressPropertiesPlanName(t, obj, planName)
		assertServiceInstanceInProgressPropertiesParameters(t, obj, inProgressParameters, inProgressParametersChecksum)
	case v1alpha1.ServiceInstanceOperationDeprovision:
		assertServiceInstanceInProgressPropertiesNil(t, obj)
	}
	assertServiceInstanceExternalPropertiesUnchanged(t, obj, originalInstance)
}

func assertServiceInstanceOperationSuccess(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, planName string, originalInstance *v1alpha1.ServiceInstance) {
	assertServiceInstanceOperationSuccessWithParameters(t, obj, operation, planName, nil, "", originalInstance)
}

func assertServiceInstanceOperationSuccessWithParameters(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, planName string, externalParameters map[string]interface{}, externalParametersChecksum string, originalInstance *v1alpha1.ServiceInstance) {
	var (
		reason      string
		readyStatus v1alpha1.ConditionStatus
	)
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		reason = successProvisionReason
		readyStatus = v1alpha1.ConditionTrue
	case v1alpha1.ServiceInstanceOperationDeprovision:
		reason = successDeprovisionReason
		readyStatus = v1alpha1.ConditionFalse
	}
	assertServiceInstanceReadyCondition(t, obj, readyStatus, reason)
	assertServiceInstanceCurrentOperationClear(t, obj)
	assertServiceInstanceOperationStartTimeSet(t, obj, false)
	assertServiceInstanceReconciledGeneration(t, obj, originalInstance.Generation)
	assertAsyncOpInProgressFalse(t, obj)
	assertServiceInstanceOrphanMitigationInProgressFalse(t, obj)
	if operation == v1alpha1.ServiceInstanceOperationDeprovision {
		assertEmptyFinalizers(t, obj)
	}
	assertServiceInstanceInProgressPropertiesNil(t, obj)
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		assertServiceInstanceExternalPropertiesPlanName(t, obj, planName)
		assertServiceInstanceExternalPropertiesParameters(t, obj, externalParameters, externalParametersChecksum)
	case v1alpha1.ServiceInstanceOperationDeprovision:
		assertServiceInstanceExternalPropertiesNil(t, obj)
	}
}

func assertServiceInstanceRequestFailingError(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, readyReason string, failureReason string, originalInstance *v1alpha1.ServiceInstance) {
	var readyStatus v1alpha1.ConditionStatus
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		readyStatus = v1alpha1.ConditionFalse
	case v1alpha1.ServiceInstanceOperationDeprovision:
		readyStatus = v1alpha1.ConditionUnknown
	}
	assertServiceInstanceReadyCondition(t, obj, readyStatus, readyReason)
	assertServiceInstanceCondition(t, obj, v1alpha1.ServiceInstanceConditionFailed, v1alpha1.ConditionTrue, failureReason)
	assertServiceInstanceOperationStartTimeSet(t, obj, false)
	assertAsyncOpInProgressFalse(t, obj)
	assertServiceInstanceInProgressPropertiesNil(t, obj)
	assertServiceInstanceExternalPropertiesUnchanged(t, obj, originalInstance)
}

func assertServiceInstanceRequestFailingErrorNoOrphanMitigation(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, readyReason string, failureReason string, originalInstance *v1alpha1.ServiceInstance) {
	assertServiceInstanceRequestFailingError(t, obj, operation, readyReason, failureReason, originalInstance)
	assertServiceInstanceCurrentOperationClear(t, obj)
	assertServiceInstanceReconciledGeneration(t, obj, originalInstance.Generation)
	assertServiceInstanceOrphanMitigationInProgressFalse(t, obj)
}

func assertServiceInstanceRequestFailingErrorStartOrphanMitigation(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, readyReason string, failureReason string, originalInstance *v1alpha1.ServiceInstance) {
	assertServiceInstanceRequestFailingError(t, obj, operation, readyReason, failureReason, originalInstance)
	assertServiceInstanceCurrentOperation(t, obj, v1alpha1.ServiceInstanceOperationProvision)
	assertServiceInstanceReconciledGeneration(t, obj, originalInstance.Status.ReconciledGeneration)
	assertServiceInstanceOrphanMitigationInProgressTrue(t, obj)
}

func assertServiceInstanceRequestRetriableError(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, reason string, planName string, originalInstance *v1alpha1.ServiceInstance) {
	assertServiceInstanceRequestRetriableErrorWithParameters(t, obj, operation, reason, planName, nil, "", originalInstance)
}

func assertServiceInstanceRequestRetriableErrorWithParameters(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, reason string, planName string, inProgressParameters map[string]interface{}, inProgressParametersChecksum string, originalInstance *v1alpha1.ServiceInstance) {
	var readyStatus v1alpha1.ConditionStatus
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		readyStatus = v1alpha1.ConditionFalse
	case v1alpha1.ServiceInstanceOperationDeprovision:
		readyStatus = v1alpha1.ConditionUnknown
	}
	assertServiceInstanceReadyCondition(t, obj, readyStatus, reason)
	assertServiceInstanceCurrentOperation(t, obj, operation)
	assertServiceInstanceOperationStartTimeSet(t, obj, true)
	assertServiceInstanceReconciledGeneration(t, obj, originalInstance.Status.ReconciledGeneration)
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		assertServiceInstanceInProgressPropertiesPlanName(t, obj, planName)
		assertServiceInstanceInProgressPropertiesParameters(t, obj, inProgressParameters, inProgressParametersChecksum)
	case v1alpha1.ServiceInstanceOperationDeprovision:
		assertServiceInstanceInProgressPropertiesNil(t, obj)
	}
	assertServiceInstanceExternalPropertiesUnchanged(t, obj, originalInstance)
}

func assertServiceInstanceAsyncInProgress(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, operationKey string, planName string, originalInstance *v1alpha1.ServiceInstance) {
	reason := ""
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		reason = asyncProvisioningReason
	case v1alpha1.ServiceInstanceOperationDeprovision:
		reason = asyncDeprovisioningReason
	}
	assertServiceInstanceReadyFalse(t, obj, reason)
	assertServiceInstanceLastOperation(t, obj, operationKey)
	assertServiceInstanceCurrentOperation(t, obj, operation)
	assertServiceInstanceOperationStartTimeSet(t, obj, true)
	assertServiceInstanceReconciledGeneration(t, obj, originalInstance.Status.ReconciledGeneration)
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		assertServiceInstanceInProgressPropertiesPlanName(t, obj, planName)
		assertServiceInstanceInProgressPropertiesParameters(t, obj, nil, "")
	case v1alpha1.ServiceInstanceOperationDeprovision:
		assertServiceInstanceInProgressPropertiesNil(t, obj)
	}
	assertAsyncOpInProgressTrue(t, obj)
}

func assertServiceInstanceConditionHasLastOperationDescription(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceOperation, lastOperationDescription string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	var expected string
	switch operation {
	case v1alpha1.ServiceInstanceOperationProvision:
		expected = fmt.Sprintf("%s (%s)", asyncProvisioningMessage, lastOperationDescription)
	case v1alpha1.ServiceInstanceOperationDeprovision:
		expected = fmt.Sprintf("%s (%s)", asyncDeprovisioningMessage, lastOperationDescription)
	}
	if e, a := expected, instance.Status.Conditions[0].Message; e != a {
		fatalf(t, "unexpected condition message: expected %q, got %q", e, a)
	}
}

func assertServiceInstanceInProgressPropertiesNil(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	if a := instance.Status.InProgressProperties; a != nil {
		fatalf(t, "expected in-progress properties to be nil: actual %v", a)
	}
}

func assertServiceInstanceInProgressPropertiesPlanName(t *testing.T, obj runtime.Object, planName string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	assertServiceInstancePropertiesStatePlanName(t, "in-progress", instance.Status.InProgressProperties, planName)
}

func assertServiceInstanceInProgressPropertiesParameters(t *testing.T, obj runtime.Object, params map[string]interface{}, checksum string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	assertServiceInstancePropertiesStateParameters(t, "in-progress", instance.Status.InProgressProperties, params, checksum)
}

func assertServiceInstanceInProgressPropertiesUnchanged(t *testing.T, obj runtime.Object, originalInstance *v1alpha1.ServiceInstance) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	if originalInstance.Status.InProgressProperties == nil {
		assertServiceInstanceInProgressPropertiesNil(t, obj)
	} else {
		assertServiceInstancePropertiesStateParametersUnchanged(t, "in-progress", instance.Status.InProgressProperties, *originalInstance.Status.InProgressProperties)
	}
}

func assertServiceInstanceExternalPropertiesNil(t *testing.T, obj runtime.Object) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}

	if a := instance.Status.ExternalProperties; a != nil {
		fatalf(t, "expected external properties to be nil: actual %v", a)
	}
}

func assertServiceInstanceExternalPropertiesPlanName(t *testing.T, obj runtime.Object, planName string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	assertServiceInstancePropertiesStatePlanName(t, "external", instance.Status.ExternalProperties, planName)
}

func assertServiceInstanceExternalPropertiesParameters(t *testing.T, obj runtime.Object, params map[string]interface{}, checksum string) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	assertServiceInstancePropertiesStateParameters(t, "external", instance.Status.ExternalProperties, params, checksum)
}

func assertServiceInstanceExternalPropertiesUnchanged(t *testing.T, obj runtime.Object, originalInstance *v1alpha1.ServiceInstance) {
	instance, ok := obj.(*v1alpha1.ServiceInstance)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstance", obj)
	}
	if originalInstance.Status.ExternalProperties == nil {
		assertServiceInstanceExternalPropertiesNil(t, obj)
	} else {
		assertServiceInstancePropertiesStateParametersUnchanged(t, "external", instance.Status.ExternalProperties, *originalInstance.Status.ExternalProperties)
	}
}

func assertServiceInstancePropertiesStatePlanName(t *testing.T, propsLabel string, actualProps *v1alpha1.ServiceInstancePropertiesState, expectedPlanName string) {
	if actualProps == nil {
		fatalf(t, "expected %v properties to not be nil", propsLabel)
	}
	if e, a := expectedPlanName, actualProps.ExternalServicePlanName; e != a {
		fatalf(t, "unexpected %v properties external service plan name: expected %v, actual %v", propsLabel, e, a)
	}
}

func assertServiceInstancePropertiesStateParameters(t *testing.T, propsLabel string, actualProps *v1alpha1.ServiceInstancePropertiesState, expectedParams map[string]interface{}, expectedChecksum string) {
	if actualProps == nil {
		fatalf(t, "expected %v properties to not be nil", propsLabel)
	}
	assertPropertiesStateParameters(t, propsLabel, actualProps.Parameters, expectedParams)
	if e, a := expectedChecksum, actualProps.ParametersChecksum; e != a {
		fatalf(t, "unexpected %v properties parameters checksum: expected %v, actual %v", propsLabel, e, a)
	}
}

func assertPropertiesStateParameters(t *testing.T, propsLabel string, marshalledParams *runtime.RawExtension, expectedParams map[string]interface{}) {
	if expectedParams == nil {
		if a := marshalledParams; a != nil {
			fatalf(t, "expected %v properties parameters to be nil: actual %v", propsLabel, a)
		}
	} else {
		if marshalledParams == nil {
			fatalf(t, "expected %v properties parameters to not be nil", propsLabel)
		}
		actualParams := make(map[string]interface{})
		if err := yaml.Unmarshal(marshalledParams.Raw, &actualParams); err != nil {
			fatalf(t, "%v properties parameters could not be unmarshalled: %v", propsLabel, err)
		}
		if e, a := expectedParams, actualParams; !reflect.DeepEqual(e, a) {
			fatalf(t, "unexpected %v properties parameters: expected %v, actual %v", propsLabel, e, a)
		}
	}
}

func assertServiceInstancePropertiesStateParametersUnchanged(t *testing.T, propsLabel string, new *v1alpha1.ServiceInstancePropertiesState, old v1alpha1.ServiceInstancePropertiesState) {
	if new == nil {
		fatalf(t, "expected %v properties to not be nil", propsLabel)
	}
	if e, a := old, *new; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected %v properties: expected %v, actual %v", propsLabel, e, a)
	}
}

func assertServiceInstanceCredentialReadyTrue(t *testing.T, obj runtime.Object) {
	assertServiceInstanceCredentialReadyCondition(t, obj, v1alpha1.ConditionTrue)
}

func assertServiceInstanceCredentialReadyFalse(t *testing.T, obj runtime.Object, reason ...string) {
	assertServiceInstanceCredentialReadyCondition(t, obj, v1alpha1.ConditionFalse, reason...)
}

func assertServiceInstanceCredentialReadyCondition(t *testing.T, obj runtime.Object, status v1alpha1.ConditionStatus, reason ...string) {
	assertServiceInstanceCredentialCondition(t, obj, v1alpha1.ServiceInstanceCredentialConditionReady, status, reason...)
}

func assertServiceInstanceCredentialCondition(t *testing.T, obj runtime.Object, conditionType v1alpha1.ServiceInstanceCredentialConditionType, status v1alpha1.ConditionStatus, reason ...string) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}

	conditionFound := false
	for _, condition := range binding.Status.Conditions {
		if condition.Type == conditionType {
			conditionFound = true
			if condition.Status != status {
				t.Logf("%v condition: %+v", conditionType, condition)
				fatalf(t, "%v condition had unexpected status; expected %v, got %v", conditionType, status, condition.Status)
			}
			if len(reason) == 1 && condition.Reason != reason[0] {
				fatalf(t, "unexpected reason; expected %v, got %v", reason[0], condition.Reason)
			}
		}
	}

	if !conditionFound {
		fatalf(t, "unfound %v condition", conditionType)
	}
}

func assertServiceInstanceCredentialReconciledGeneration(t *testing.T, obj runtime.Object, reconciledGeneration int64) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}

	if e, a := reconciledGeneration, binding.Status.ReconciledGeneration; e != a {
		fatalf(t, "unexpected reconciled generation: expected %v, got %v", e, a)
	}
}

func assertServiceInstanceCredentialReconciliationNotComplete(t *testing.T, obj runtime.Object) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}
	if g, rg := binding.Generation, binding.Status.ReconciledGeneration; g <= rg {
		fatalf(t, "expected ReconciledGeneration to be less than Generation: Generation %v, ReconciledGeneration %v", g, rg)
	}
}

func assertServiceInstanceCredentialOperationStartTimeSet(t *testing.T, obj runtime.Object, isOperationStartTimeSet bool) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}

	if e, a := isOperationStartTimeSet, binding.Status.OperationStartTime != nil; e != a {
		if e {
			fatalf(t, "expected OperationStartTime to not be nil, but was")
		} else {
			fatalf(t, "expected OperationStartTime to be nil, but was not")
		}
	}
}

func assertServiceInstanceCredentialCurrentOperationClear(t *testing.T, obj runtime.Object) {
	assertServiceInstanceCredentialCurrentOperation(t, obj, "")
}

func assertServiceInstanceCredentialCurrentOperation(t *testing.T, obj runtime.Object, currentOperation v1alpha1.ServiceInstanceCredentialOperation) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}

	if e, a := currentOperation, binding.Status.CurrentOperation; e != a {
		fatalf(t, "unexpected current operation: expected %q, got %q", e, a)
	}
}

func assertServiceInstanceCredentialErrorBeforeRequest(t *testing.T, obj runtime.Object, reason string, originalBinding *v1alpha1.ServiceInstanceCredential) {
	assertServiceInstanceCredentialReadyFalse(t, obj, reason)
	assertServiceInstanceCredentialCurrentOperationClear(t, obj)
	assertServiceInstanceCredentialOperationStartTimeSet(t, obj, false)
	assertServiceInstanceCredentialReconciledGeneration(t, obj, originalBinding.Status.ReconciledGeneration)
	assertServiceInstanceCredentialInProgressPropertiesNil(t, obj)
	assertServiceInstanceCredentialExternalPropertiesUnchanged(t, obj, originalBinding)
}

func assertServiceInstanceCredentialOperationInProgress(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceCredentialOperation, originalBinding *v1alpha1.ServiceInstanceCredential) {
	assertServiceInstanceCredentialOperationInProgressWithParameters(t, obj, operation, nil, "", originalBinding)
}

func assertServiceInstanceCredentialOperationInProgressWithParameters(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceCredentialOperation, inProgressParameters map[string]interface{}, inProgressParametersChecksum string, originalBinding *v1alpha1.ServiceInstanceCredential) {
	reason := ""
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		reason = bindingInFlightReason
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		reason = unbindingInFlightReason
	}
	assertServiceInstanceCredentialReadyFalse(t, obj, reason)
	assertServiceInstanceCredentialCurrentOperation(t, obj, operation)
	assertServiceInstanceCredentialOperationStartTimeSet(t, obj, true)
	assertServiceInstanceCredentialReconciledGeneration(t, obj, originalBinding.Status.ReconciledGeneration)
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		assertServiceInstanceCredentialInProgressPropertiesParameters(t, obj, inProgressParameters, inProgressParametersChecksum)
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		assertServiceInstanceCredentialInProgressPropertiesNil(t, obj)
	}
	assertServiceInstanceCredentialExternalPropertiesUnchanged(t, obj, originalBinding)
}

func assertServiceInstanceCredentialOperationSuccess(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceCredentialOperation, originalBinding *v1alpha1.ServiceInstanceCredential) {
	assertServiceInstanceCredentialOperationSuccessWithParameters(t, obj, operation, nil, "", originalBinding)
}

func assertServiceInstanceCredentialOperationSuccessWithParameters(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceCredentialOperation, externalParameters map[string]interface{}, externalParametersChecksum string, originalBinding *v1alpha1.ServiceInstanceCredential) {
	var (
		reason      string
		readyStatus v1alpha1.ConditionStatus
	)
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		reason = successInjectedBindResultReason
		readyStatus = v1alpha1.ConditionTrue
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		reason = successUnboundReason
		readyStatus = v1alpha1.ConditionFalse
	}
	assertServiceInstanceCredentialReadyCondition(t, obj, readyStatus, reason)
	assertServiceInstanceCredentialCurrentOperationClear(t, obj)
	assertServiceInstanceCredentialOperationStartTimeSet(t, obj, false)
	assertServiceInstanceCredentialReconciledGeneration(t, obj, originalBinding.Generation)
	if operation == v1alpha1.ServiceInstanceCredentialOperationUnbind {
		assertEmptyFinalizers(t, obj)
	}
	assertServiceInstanceCredentialInProgressPropertiesNil(t, obj)
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		assertServiceInstanceCredentialExternalPropertiesParameters(t, obj, externalParameters, externalParametersChecksum)
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		assertServiceInstanceCredentialExternalPropertiesNil(t, obj)
	}
}

func assertServiceInstanceCredentialRequestFailingError(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceCredentialOperation, readyReason string, failureReason string, originalBinding *v1alpha1.ServiceInstanceCredential) {
	var readyStatus v1alpha1.ConditionStatus
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		readyStatus = v1alpha1.ConditionFalse
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		readyStatus = v1alpha1.ConditionUnknown
	}
	assertServiceInstanceCredentialReadyCondition(t, obj, readyStatus, readyReason)
	assertServiceInstanceCredentialCondition(t, obj, v1alpha1.ServiceInstanceCredentialConditionFailed, v1alpha1.ConditionTrue, failureReason)
	assertServiceInstanceCredentialCurrentOperationClear(t, obj)
	assertServiceInstanceCredentialOperationStartTimeSet(t, obj, false)
	assertServiceInstanceCredentialReconciledGeneration(t, obj, originalBinding.Generation)
	assertServiceInstanceCredentialInProgressPropertiesNil(t, obj)
	assertServiceInstanceCredentialExternalPropertiesUnchanged(t, obj, originalBinding)
}

func assertServiceInstanceCredentialRequestRetriableError(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceCredentialOperation, reason string, originalBinding *v1alpha1.ServiceInstanceCredential) {
	assertServiceInstanceCredentialRequestRetriableErrorWithParameters(t, obj, operation, reason, nil, "", originalBinding)
}

func assertServiceInstanceCredentialRequestRetriableErrorWithParameters(t *testing.T, obj runtime.Object, operation v1alpha1.ServiceInstanceCredentialOperation, reason string, inProgressParameters map[string]interface{}, inProgressParametersChecksum string, originalBinding *v1alpha1.ServiceInstanceCredential) {
	var readyStatus v1alpha1.ConditionStatus
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		readyStatus = v1alpha1.ConditionFalse
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		readyStatus = v1alpha1.ConditionUnknown
	}
	assertServiceInstanceCredentialReadyCondition(t, obj, readyStatus, reason)
	assertServiceInstanceCredentialCurrentOperation(t, obj, operation)
	assertServiceInstanceCredentialOperationStartTimeSet(t, obj, true)
	assertServiceInstanceCredentialReconciledGeneration(t, obj, originalBinding.Status.ReconciledGeneration)
	switch operation {
	case v1alpha1.ServiceInstanceCredentialOperationBind:
		assertServiceInstanceCredentialInProgressPropertiesParameters(t, obj, inProgressParameters, inProgressParametersChecksum)
	case v1alpha1.ServiceInstanceCredentialOperationUnbind:
		assertServiceInstanceCredentialInProgressPropertiesNil(t, obj)
	}
	assertServiceInstanceCredentialExternalPropertiesUnchanged(t, obj, originalBinding)
}

func assertServiceInstanceCredentialInProgressPropertiesNil(t *testing.T, obj runtime.Object) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}

	if a := binding.Status.InProgressProperties; a != nil {
		fatalf(t, "expected in-progress properties to be nil: actual %v", a)
	}
}

func assertServiceInstanceCredentialInProgressPropertiesParameters(t *testing.T, obj runtime.Object, params map[string]interface{}, checksum string) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}
	assertServiceInstanceCredentialPropertiesStateParameters(t, "in-progress", binding.Status.InProgressProperties, params, checksum)
}

func assertServiceInstanceCredentialInProgressPropertiesUnchanged(t *testing.T, obj runtime.Object, originalBinding *v1alpha1.ServiceInstanceCredential) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}
	if originalBinding.Status.InProgressProperties == nil {
		assertServiceInstanceCredentialInProgressPropertiesNil(t, obj)
	} else {
		assertServiceInstanceCredentialPropertiesStateParametersUnchanged(t, "in-progress", binding.Status.InProgressProperties, *originalBinding.Status.InProgressProperties)
	}
}

func assertServiceInstanceCredentialExternalPropertiesNil(t *testing.T, obj runtime.Object) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}

	if a := binding.Status.ExternalProperties; a != nil {
		fatalf(t, "expected external properties to be nil: actual %v", a)
	}
}

func assertServiceInstanceCredentialExternalPropertiesParameters(t *testing.T, obj runtime.Object, params map[string]interface{}, checksum string) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}
	assertServiceInstanceCredentialPropertiesStateParameters(t, "external", binding.Status.ExternalProperties, params, checksum)
}

func assertServiceInstanceCredentialExternalPropertiesUnchanged(t *testing.T, obj runtime.Object, originalBinding *v1alpha1.ServiceInstanceCredential) {
	binding, ok := obj.(*v1alpha1.ServiceInstanceCredential)
	if !ok {
		fatalf(t, "Couldn't convert object %+v into a *v1alpha1.ServiceInstanceCredential", obj)
	}
	if originalBinding.Status.ExternalProperties == nil {
		assertServiceInstanceCredentialExternalPropertiesNil(t, obj)
	} else {
		assertServiceInstanceCredentialPropertiesStateParametersUnchanged(t, "external", binding.Status.ExternalProperties, *originalBinding.Status.ExternalProperties)
	}
}

func assertServiceInstanceCredentialPropertiesStateParameters(t *testing.T, propsLabel string, actualProps *v1alpha1.ServiceInstanceCredentialPropertiesState, expectedParams map[string]interface{}, expectedChecksum string) {
	if actualProps == nil {
		fatalf(t, "expected %v properties to not be nil", propsLabel)
	}
	assertPropertiesStateParameters(t, propsLabel, actualProps.Parameters, expectedParams)
	if e, a := expectedChecksum, actualProps.ParametersChecksum; e != a {
		fatalf(t, "unexpected %v properties parameters checksum: expected %v, actual %v", propsLabel, e, a)
	}
}

func assertServiceInstanceCredentialPropertiesStateParametersUnchanged(t *testing.T, propsLabel string, new *v1alpha1.ServiceInstanceCredentialPropertiesState, old v1alpha1.ServiceInstanceCredentialPropertiesState) {
	if new == nil {
		fatalf(t, "expected %v properties to not be nil", propsLabel)
	}
	if e, a := old, *new; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected %v properties: expected %v, actual %v", propsLabel, e, a)
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

func assertCatalogFinalizerExists(t *testing.T, obj runtime.Object) {
	testCatalogFinalizerExists(t, "" /* name */, fatalf, obj)
}

func expectCatalogFinalizerExists(t *testing.T, name string, obj runtime.Object) bool {
	return testCatalogFinalizerExists(t, name, errorf, obj)
}

func testCatalogFinalizerExists(t *testing.T, name string, f failfFunc, obj runtime.Object) bool {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		f(t, "%vError creating ObjectMetaAccessor for param object %+v: %v", logContext, obj, err)
		return false
	}

	finalizers := sets.NewString(accessor.GetFinalizers()...)
	if !finalizers.Has(v1alpha1.FinalizerServiceCatalog) {
		f(t, "%vExpected Service Catalog finalizer but was not there", logContext)
		return false
	}

	return true
}

func assertNumberOfServiceBrokerActions(t *testing.T, actions []fakeosb.Action, number int) {
	testNumberOfServiceBrokerActions(t, "" /* name */, fatalf, actions, number)
}

// assertListRestrictions compares expected Fields / Labels on a list options.
func assertListRestrictions(t *testing.T, e, a clientgotesting.ListRestrictions) {
	if el, al := e.Labels.String(), a.Labels.String(); el != al {
		fatalf(t, "ListRestrictions.Labels don't match, expected %q got %q", el, al)
	}
	if ef, af := e.Fields.String(), a.Fields.String(); ef != af {
		fatalf(t, "ListRestrictions.Fields don't match, expected %q got %q", ef, af)
	}
}

func expectNumberOfServiceBrokerActions(t *testing.T, name string, actions []fakeosb.Action, number int) bool {
	return testNumberOfServiceBrokerActions(t, name, errorf, actions, number)
}

func testNumberOfServiceBrokerActions(t *testing.T, name string, f failfFunc, actions []fakeosb.Action, number int) bool {
	logContext := ""
	if len(name) > 0 {
		logContext = name + ": "
	}

	if e, a := number, len(actions); e != a {
		t.Logf("%+v\n", actions)
		f(t, "%vUnexpected number of actions: expected %v, got %v\nactions: %+v", logContext, e, a, actions)
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

	actualRequest, ok := action.Request.(*osb.BindRequest)
	if !ok {
		fatalf(t, "unexpected request type; expected %T, got %T", request, actualRequest)
	}

	expectedOriginatingIdentity := request.OriginatingIdentity
	actualOriginatingIdentity := actualRequest.OriginatingIdentity
	request.OriginatingIdentity = nil
	actualRequest.OriginatingIdentity = nil

	if e, a := request, action.Request; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in bind request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
	}

	request.OriginatingIdentity = expectedOriginatingIdentity
	actualRequest.OriginatingIdentity = actualOriginatingIdentity

	assertOriginatingIdentity(t, expectedOriginatingIdentity, actualOriginatingIdentity)
}

func assertUnbind(t *testing.T, action fakeosb.Action, request *osb.UnbindRequest) {
	if e, a := fakeosb.Unbind, action.Type; e != a {
		fatalf(t, "unexpected action type; expected %v, got %v", e, a)
	}

	if e, a := request, action.Request; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in bind request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
	}
}

func assertOriginatingIdentity(t *testing.T, expected *osb.AlphaOriginatingIdentity, actual *osb.AlphaOriginatingIdentity) {
	if e, a := expected, actual; (e != nil) != (a != nil) {
		fatalf(t, "unexpected originating identity in request: expected %q, got %q", e, a)
	}
	if expected == nil {
		return
	}
	if e, a := expected.Platform, actual.Platform; e != a {
		fatalf(t, "invalid originating identity platform in request: expected %q, got %q", e, a)
	}
	var expectedValue interface{}
	if err := json.Unmarshal([]byte(expected.Value), &expectedValue); err != nil {
		fatalf(t, "invalid originating identity value in expected request: %q: %v", expected.Value, err)
	}
	var actualValue interface{}
	if err := json.Unmarshal([]byte(actual.Value), &actualValue); err != nil {
		fatalf(t, "invalid originating identity value in actual request: %q: %v", actual.Value, err)
	}
	if e, a := expectedValue, actualValue; !reflect.DeepEqual(e, a) {
		fatalf(t, "unexpected diff in originating identity value in request: %v\nexpected %+v\ngot      %+v", diff.ObjectReflectDiff(e, a), e, a)
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
