/*
Copyright 2018 The Kubernetes Authors.

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

package servicecatalog_test

import (
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/testing"

	. "github.com/kubernetes-incubator/service-catalog/pkg/svcat/service-catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Instances", func() {
	var (
		sdk          *SDK
		svcCatClient *fake.Clientset
		si           *v1beta1.ServiceInstance
		si2          *v1beta1.ServiceInstance
	)

	BeforeEach(func() {
		si = &v1beta1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: "foobar", Namespace: "foobar_namespace"}}
		si2 = &v1beta1.ServiceInstance{ObjectMeta: metav1.ObjectMeta{Name: "barbaz", Namespace: "foobar_namespace"}}
		svcCatClient = fake.NewSimpleClientset(si, si2)
		sdk = &SDK{
			ServiceCatalogClient: svcCatClient,
		}
	})

	Describe("RetrieveInstancees", func() {
		It("Calls the generated v1beta1 List method with the specified namespace", func() {
			namespace := si.Namespace

			instances, err := sdk.RetrieveInstances(namespace)

			Expect(err).NotTo(HaveOccurred())
			Expect(instances.Items).Should(ConsistOf(*si, *si2))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("list", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.ListActionImpl).Namespace).To(Equal(namespace))
		})
		It("Bubbles up errors", func() {
			namespace := si.Namespace
			badClient := &fake.Clientset{}
			errorMessage := "error retrieving list"
			badClient.AddReactor("list", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			_, err := sdk.RetrieveInstances(namespace)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errorMessage))
			Expect(badClient.Actions()[0].Matches("list", "serviceinstances")).To(BeTrue())
		})
	})
	Describe("RetrieveInstance", func() {
		It("Calls the generated v1beta1 Get method with the passed in instance", func() {
			instanceName := si.Name
			namespace := si.Namespace

			instance, err := sdk.RetrieveInstance(namespace, instanceName)
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).To(Equal(si))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
		It("Bubbles up errors", func() {
			instanceName := "not_real"
			namespace := "foobar_namespace"

			_, err := sdk.RetrieveInstance(namespace, instanceName)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
	})
	Describe("RetrieveInstanceByBinding", func() {
		It("Calls the generated v1beta1 Get method with the binding's namespace and the binding's instance's name", func() {
			instanceName := si.Name
			namespace := si.Namespace
			sb := &v1beta1.ServiceBinding{ObjectMeta: metav1.ObjectMeta{Name: "banana_binding", Namespace: namespace}}
			sb.Spec.ServiceInstanceRef.Name = instanceName
			instance, err := sdk.RetrieveInstanceByBinding(sb)

			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
			Expect(instance).To(Equal(si))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
		It("Bubbles up errors", func() {
			namespace := si.Namespace
			instanceName := "not_real_instance"
			sb := &v1beta1.ServiceBinding{ObjectMeta: metav1.ObjectMeta{Name: "banana_binding", Namespace: namespace}}
			sb.Spec.ServiceInstanceRef.Name = instanceName
			badClient := &fake.Clientset{}
			errorMessage := "no instance found"
			badClient.AddReactor("get", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient
			instance, err := sdk.RetrieveInstanceByBinding(sb)
			Expect(instance).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
			actions := badClient.Actions()
			Expect(actions[0].Matches("get", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(instanceName))
			Expect(actions[0].(testing.GetActionImpl).Namespace).To(Equal(namespace))
		})
	})
	Describe("RetrieveInstancesByPlan", func() {
		It("Calls the generated v1beta1 List method with a ListOption containing the passed in plan", func() {
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: plan.Name,
					},
				},
			}
			linkedClient := fake.NewSimpleClientset(si, si2)
			sdk.ServiceCatalogClient = linkedClient

			_, err := sdk.RetrieveInstancesByPlan(plan)
			Expect(err).NotTo(HaveOccurred())
			actions := linkedClient.Actions()
			Expect(actions[0].Matches("list", "serviceinstances")).To(BeTrue())
			opts := fields.Set{"spec.clusterServicePlanRef.name": plan.Name}
			Expect(actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Matches(opts)).To(BeTrue())
		})
		It("Bubbles up errors", func() {
			badClient := &fake.Clientset{}
			errorMessage := "no instances found"
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			badClient.AddReactor("list", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			instances, err := sdk.RetrieveInstancesByPlan(plan)
			Expect(instances).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
			actions := badClient.Actions()
			Expect(actions[0].Matches("list", "serviceinstances")).To(BeTrue())
			opts := fields.Set{"spec.clusterServicePlanRef.name": plan.Name}
			Expect(actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Matches(opts)).To(BeTrue())
		})
	})
	Describe("InstanceParentHierarchy", func() {
		It("calls the v1beta1 generated Get function repeatedly to build the heirarchy of the passed in service isntance", func() {
			broker := &v1beta1.ClusterServiceBroker{ObjectMeta: metav1.ObjectMeta{Name: "foobar_broker"}}
			class := &v1beta1.ClusterServiceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_class",
				},
				Spec: v1beta1.ClusterServiceClassSpec{
					ClusterServiceBrokerName: broker.Name,
				},
			}
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: plan.Name,
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: class.Name,
					},
				},
			}
			linkedClient := fake.NewSimpleClientset(si, si2, class, plan, broker)
			sdk.ServiceCatalogClient = linkedClient
			retClass, retPlan, retBroker, err := sdk.InstanceParentHierarchy(si)
			Expect(err).NotTo(HaveOccurred())
			Expect(retClass.Name).To(Equal(class.Name))
			Expect(retPlan.Name).To(Equal(plan.Name))
			Expect(retBroker.Name).To(Equal(broker.Name))
			actions := linkedClient.Actions()
			getClass := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceclasses",
					},
				},
				Name: class.Name,
			}
			getPlan := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceplans",
					},
				},
				Name: plan.Name,
			}
			getBroker := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterservicebrokers",
					},
				},
				Name: broker.Name,
			}
			Expect(actions).Should(ContainElement(getClass))
			Expect(actions).Should(ContainElement(getPlan))
			Expect(actions).Should(ContainElement(getBroker))
		})
		It("Bubbles up errors", func() {
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_plan",
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_class",
					},
				},
			}
			badClient := &fake.Clientset{}
			errorMessage := "error retrieving thing"
			badClient.AddReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			badClient.AddReactor("get", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			a, b, c, err := sdk.InstanceParentHierarchy(si)
			Expect(a).To(BeNil())
			Expect(b).To(BeNil())
			Expect(c).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
		})
	})
	Describe("InstanceToServiceClassAndPlan", func() {
		It("Calls the generated v1beta methods with the names of the class and plan from the passed in instance", func() {
			class := &v1beta1.ClusterServiceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_class",
				},
			}
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{},
			}
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: plan.Name,
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: class.Name,
					},
				},
			}
			linkedClient := fake.NewSimpleClientset(si, si2, class, plan)
			sdk.ServiceCatalogClient = linkedClient

			retClass, retPlan, err := sdk.InstanceToServiceClassAndPlan(si)
			Expect(err).NotTo(HaveOccurred())
			Expect(retClass).To(Equal(class))
			Expect(retPlan).To(Equal(plan))
			actions := linkedClient.Actions()
			getClass := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceclasses",
					},
				},
				Name: class.Name,
			}
			getPlan := testing.GetActionImpl{
				ActionImpl: testing.ActionImpl{
					Verb: "get",
					Resource: schema.GroupVersionResource{
						Group:    "servicecatalog.k8s.io",
						Version:  "v1beta1",
						Resource: "clusterserviceplans",
					},
				},
				Name: plan.Name,
			}
			Expect(actions).Should(ContainElement(getClass))
			Expect(actions).Should(ContainElement(getPlan))
		})
		It("Bubbles up errors", func() {
			si = &v1beta1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foobar",
					Namespace: "foobar_namespace",
				},
				Spec: v1beta1.ServiceInstanceSpec{
					ClusterServicePlanRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_plan",
					},
					ClusterServiceClassRef: &v1beta1.ClusterObjectReference{
						Name: "not_real_class",
					},
				},
			}
			badClient := &fake.Clientset{}
			errorMessage := "error retrieving thing"
			badClient.AddReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			badClient.AddReactor("get", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			a, b, err := sdk.InstanceToServiceClassAndPlan(si)
			Expect(a).To(BeNil())
			Expect(b).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
		})
	})
	Describe("Provision", func() {
		It("Calls the v1beta1 Create method with the passed in arguements", func() {
			namespace := "cherry_namespace"
			instanceName := "cherry"
			className := "cherry_class"
			planName := "cherry_plan"
			params := make(map[string]string)
			params["foo"] = "bar"
			secrets := make(map[string]string)
			secrets["username"] = "admin"
			secrets["password"] = "abc123"

			service, err := sdk.Provision(namespace, instanceName, className, planName, params, secrets)

			Expect(err).NotTo(HaveOccurred())
			Expect(service.Namespace).To(Equal(namespace))
			Expect(service.Name).To(Equal(instanceName))
			Expect(service.Spec.PlanReference.ClusterServiceClassExternalName).To(Equal(className))
			Expect(service.Spec.PlanReference.ClusterServicePlanExternalName).To(Equal(planName))

			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("create", "serviceinstances")).To(BeTrue())
			objectFromRequest := actions[0].(testing.CreateActionImpl).Object.(*v1beta1.ServiceInstance)
			Expect(objectFromRequest.ObjectMeta.Name).To(Equal(instanceName))
			Expect(objectFromRequest.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(objectFromRequest.Spec.PlanReference.ClusterServiceClassExternalName).To(Equal(className))
			Expect(objectFromRequest.Spec.PlanReference.ClusterServicePlanExternalName).To(Equal(planName))
			Expect(objectFromRequest.Spec.Parameters.Raw).To(Equal([]byte("{\"foo\":\"bar\"}")))
			param := v1beta1.ParametersFromSource{
				SecretKeyRef: &v1beta1.SecretKeyReference{
					Name: "username",
					Key:  "admin",
				},
			}
			param2 := v1beta1.ParametersFromSource{
				SecretKeyRef: &v1beta1.SecretKeyReference{
					Name: "password",
					Key:  "abc123",
				},
			}
			Expect(objectFromRequest.Spec.ParametersFrom).Should(ConsistOf(param, param2))
		})
		It("Bubbles up errors", func() {
			errorMessage := "error retrieving list"
			namespace := "cherry_namespace"
			instanceName := "cherry"
			className := "cherry_class"
			planName := "cherry_plan"
			params := make(map[string]string)
			params["foo"] = "bar"
			secrets := make(map[string]string)
			secrets["username"] = "admin"
			secrets["password"] = "abc123"
			badClient := &fake.Clientset{}
			badClient.AddReactor("create", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			service, err := sdk.Provision(namespace, instanceName, className, planName, params, secrets)
			Expect(service).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
		})
	})
	Describe("Deprovision", func() {
		It("Calls the v1beta1 Delete method wiht the passed in service instance name", func() {
			err := sdk.Deprovision(si.Namespace, si.Name)
			Expect(err).NotTo(HaveOccurred())
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("delete", "serviceinstances")).To(BeTrue())
			Expect(actions[0].(testing.DeleteActionImpl).Name).To(Equal(si.Name))
		})
	})
	It("Bubbles up errors", func() {
		errorMessage := "instance not found"
		badClient := &fake.Clientset{}
		badClient.AddReactor("delete", "serviceinstances", func(action testing.Action) (bool, runtime.Object, error) {
			return true, nil, fmt.Errorf(errorMessage)
		})
		sdk.ServiceCatalogClient = badClient

		err := sdk.Deprovision(si.Namespace, si.Name)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring(errorMessage))
		actions := badClient.Actions()
		Expect(actions[0].Matches("delete", "serviceinstances")).To(BeTrue())
		Expect(actions[0].(testing.DeleteActionImpl).Name).To(Equal(si.Name))
	})
})
