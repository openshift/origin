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

package e2e

import (
	v1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"github.com/kubernetes-incubator/service-catalog/test/e2e/framework"
	"github.com/kubernetes-incubator/service-catalog/test/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = framework.ServiceCatalogDescribe("walkthrough", func() {
	f := framework.NewDefaultFramework("walkthrough-example")

	upsbrokername := "ups-broker"

	BeforeEach(func() {
		// Deploy the ups-broker
		By("Creating a ups-broker pod")
		pod, err := f.KubeClientSet.CoreV1().Pods(f.Namespace.Name).Create(NewUPSBrokerPod(upsbrokername))
		Expect(err).NotTo(HaveOccurred(), "failed to create upsbroker pod")

		By("Waiting for ups-broker pod to be running")
		err = framework.WaitForPodRunningInNamespace(f.KubeClientSet, pod)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a ups-broker service")
		_, err = f.KubeClientSet.CoreV1().Services(f.Namespace.Name).Create(NewUPSBrokerService(upsbrokername))
		Expect(err).NotTo(HaveOccurred(), "failed to create upsbroker service")

		By("Waiting for service endpoint")
		err = framework.WaitForEndpoint(f.KubeClientSet, f.Namespace.Name, upsbrokername)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		// Delete ups-broker pod and service
		By("Deleting the ups-broker pod")
		err := f.KubeClientSet.CoreV1().Pods(f.Namespace.Name).Delete(upsbrokername, nil)
		Expect(err).NotTo(HaveOccurred())

		By("Deleting the ups-broker service")
		err = f.KubeClientSet.CoreV1().Services(f.Namespace.Name).Delete(upsbrokername, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Run walkthrough-example ", func() {
		var (
			brokerName              = upsbrokername
			serviceclassName        = "user-provided-service"
			serviceclassID          = "4f6e6cf6-ffdd-425f-a2c7-3c9258ad2468"
			serviceplanID           = "86064792-7ea2-467b-af93-ac9694d96d52"
			testns                  = "test-ns"
			instanceName            = "ups-instance"
			bindingName             = "ups-binding"
			instanceNameDef         = "ups-instance-def"
			instanceNameK8sNames    = "ups-instance-k8s-names"
			instanceNameK8sNamesDef = "ups-instance-k8s-names-def"
		)

		// Broker and ClusterServiceClass should become ready
		By("Make sure the named ClusterServiceBroker does not exist before create")
		if _, err := f.ServiceCatalogClientSet.ServicecatalogV1beta1().ClusterServiceBrokers().Get(brokerName, metav1.GetOptions{}); err == nil {
			err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ClusterServiceBrokers().Delete(brokerName, nil)
			Expect(err).NotTo(HaveOccurred(), "failed to delete the broker")

			By("Waiting for ClusterServiceBroker to not exist")
			err = util.WaitForBrokerToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), brokerName)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Creating a ClusterServiceBroker")
		url := "http://" + upsbrokername + "." + f.Namespace.Name + ".svc.cluster.local"
		broker := &v1beta1.ClusterServiceBroker{
			ObjectMeta: metav1.ObjectMeta{
				Name: brokerName,
			},
			Spec: v1beta1.ClusterServiceBrokerSpec{
				URL: url,
			},
		}
		broker, err := f.ServiceCatalogClientSet.ServicecatalogV1beta1().ClusterServiceBrokers().Create(broker)
		Expect(err).NotTo(HaveOccurred(), "failed to create ClusterServiceBroker")

		By("Waiting for ClusterServiceBroker to be ready")
		err = util.WaitForBrokerCondition(f.ServiceCatalogClientSet.ServicecatalogV1beta1(),
			broker.Name,
			v1beta1.ServiceBrokerCondition{
				Type:   v1beta1.ServiceBrokerConditionReady,
				Status: v1beta1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait ClusterServiceBroker to be ready")

		By("Waiting for ClusterServiceClass to be ready")
		err = util.WaitForClusterServiceClassToExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), serviceclassID)
		Expect(err).NotTo(HaveOccurred(), "failed to wait serviceclass to be ready")

		// Provisioning a ServiceInstance and binding to it
		By("Creating a namespace")
		testnamespace, err := framework.CreateKubeNamespace(testns, f.KubeClientSet)
		Expect(err).NotTo(HaveOccurred(), "failed to create kube namespace")

		By("Creating a ServiceInstance")
		instance := &v1beta1.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceName,
				Namespace: testnamespace.Name,
			},
			Spec: v1beta1.ServiceInstanceSpec{
				PlanReference: v1beta1.PlanReference{
					ExternalClusterServiceClassName: serviceclassName,
					ExternalClusterServicePlanName:  "default",
				},
			},
		}
		instance, err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Create(instance)
		Expect(err).NotTo(HaveOccurred(), "failed to create instance")
		Expect(instance).NotTo(BeNil())

		By("Waiting for ServiceInstance to be ready")
		err = util.WaitForInstanceCondition(f.ServiceCatalogClientSet.ServicecatalogV1beta1(),
			testnamespace.Name,
			instanceName,
			v1beta1.ServiceInstanceCondition{
				Type:   v1beta1.ServiceInstanceConditionReady,
				Status: v1beta1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait instance to be ready")

		// Make sure references have been resolved
		By("References should have been resolved before ServiceInstance is ready ")
		sc, err := f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Get(instanceName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to get ServiceInstance after binding")
		Expect(sc.Spec.ClusterServiceClassRef).NotTo(BeNil())
		Expect(sc.Spec.ClusterServicePlanRef).NotTo(BeNil())
		Expect(sc.Spec.ClusterServiceClassRef.Name).To(Equal(serviceclassID))
		Expect(sc.Spec.ClusterServicePlanRef.Name).To(Equal(serviceplanID))

		// Binding to the ServiceInstance
		By("Creating a ServiceBinding")
		binding := &v1beta1.ServiceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: testnamespace.Name,
			},
			Spec: v1beta1.ServiceBindingSpec{
				ServiceInstanceRef: corev1.LocalObjectReference{
					Name: instanceName,
				},
				SecretName: "my-secret",
			},
		}
		binding, err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceBindings(testnamespace.Name).Create(binding)
		Expect(err).NotTo(HaveOccurred(), "failed to create binding")
		Expect(binding).NotTo(BeNil())

		By("Waiting for ServiceBinding to be ready")
		err = util.WaitForBindingCondition(f.ServiceCatalogClientSet.ServicecatalogV1beta1(),
			testnamespace.Name,
			bindingName,
			v1beta1.ServiceBindingCondition{
				Type:   v1beta1.ServiceBindingConditionReady,
				Status: v1beta1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait binding to be ready")

		By("Secret should have been created after binding")
		_, err = f.KubeClientSet.CoreV1().Secrets(testnamespace.Name).Get("my-secret", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create secret after binding")

		// Unbinding from the ServiceInstance
		By("Deleting the ServiceBinding")
		err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceBindings(testnamespace.Name).Delete(bindingName, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the binding")

		By("Waiting for ServiceBinding to not exist")
		err = util.WaitForBindingToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), testnamespace.Name, bindingName)
		Expect(err).NotTo(HaveOccurred())

		By("Secret should been deleted after delete the binding")
		_, err = f.KubeClientSet.CoreV1().Secrets(testnamespace.Name).Get("my-secret", metav1.GetOptions{})
		Expect(err).To(HaveOccurred())

		// Deprovisioning the ServiceInstance
		By("Deleting the ServiceInstance")
		err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Delete(instanceName, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the instance")

		By("Waiting for ServiceInstance to not exist")
		err = util.WaitForInstanceToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), testnamespace.Name, instanceName)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a ServiceInstance using a default plan")
		instanceDef := &v1beta1.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceNameDef,
				Namespace: testnamespace.Name,
			},
			Spec: v1beta1.ServiceInstanceSpec{
				PlanReference: v1beta1.PlanReference{
					ExternalClusterServiceClassName: serviceclassName,
				},
			},
		}
		instance, err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Create(instanceDef)
		Expect(err).NotTo(HaveOccurred(), "failed to create instance with default plan")
		Expect(instanceDef).NotTo(BeNil())

		By("Waiting for ServiceInstance to be ready")
		err = util.WaitForInstanceCondition(f.ServiceCatalogClientSet.ServicecatalogV1beta1(),
			testnamespace.Name,
			instanceNameDef,
			v1beta1.ServiceInstanceCondition{
				Type:   v1beta1.ServiceInstanceConditionReady,
				Status: v1beta1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait instance with default plan to be ready")

		// Deprovisioning the ServiceInstance with default plan
		By("Deleting the ServiceInstance with default plan")
		err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Delete(instanceNameDef, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the instance with default plan")

		By("Waiting for ServiceInstance with default plan to not exist")
		err = util.WaitForInstanceToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), testnamespace.Name, instanceNameDef)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a ServiceInstance using k8s names plan")
		instanceK8SNames := &v1beta1.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceNameK8sNames,
				Namespace: testnamespace.Name,
			},
			Spec: v1beta1.ServiceInstanceSpec{
				PlanReference: v1beta1.PlanReference{
					ClusterServiceClassName: serviceclassID,
					ClusterServicePlanName:  serviceplanID,
				},
			},
		}
		instance, err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Create(instanceK8SNames)
		Expect(err).NotTo(HaveOccurred(), "failed to create instance with K8S names")
		Expect(instanceK8SNames).NotTo(BeNil())

		By("Waiting for ServiceInstance with k8s names to be ready")
		err = util.WaitForInstanceCondition(f.ServiceCatalogClientSet.ServicecatalogV1beta1(),
			testnamespace.Name,
			instanceNameK8sNames,
			v1beta1.ServiceInstanceCondition{
				Type:   v1beta1.ServiceInstanceConditionReady,
				Status: v1beta1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait instance with k8s names to be ready")

		// Deprovisioning the ServiceInstance with k8s names
		By("Deleting the ServiceInstance with k8s names")
		err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Delete(instanceNameK8sNames, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the instance with k8s names")

		By("Waiting for ServiceInstance with k8s names to not exist")
		err = util.WaitForInstanceToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), testnamespace.Name, instanceNameK8sNames)
		Expect(err).NotTo(HaveOccurred())

		By("Creating a ServiceInstance using k8s name and default plan")
		instanceK8SNamesDef := &v1beta1.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceNameK8sNamesDef,
				Namespace: testnamespace.Name,
			},
			Spec: v1beta1.ServiceInstanceSpec{
				PlanReference: v1beta1.PlanReference{
					ClusterServiceClassName: serviceclassID,
				},
			},
		}
		instance, err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Create(instanceK8SNamesDef)
		Expect(err).NotTo(HaveOccurred(), "failed to create instance with K8S name and default plan")
		Expect(instanceK8SNamesDef).NotTo(BeNil())

		By("Waiting for ServiceInstance with k8s name and default plan to be ready")
		err = util.WaitForInstanceCondition(f.ServiceCatalogClientSet.ServicecatalogV1beta1(),
			testnamespace.Name,
			instanceNameK8sNamesDef,
			v1beta1.ServiceInstanceCondition{
				Type:   v1beta1.ServiceInstanceConditionReady,
				Status: v1beta1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait instance with k8s name and default plan to be ready")

		// Deprovisioning the ServiceInstance with k8s name and default plan
		By("Deleting the ServiceInstance with k8s name and default plan")
		err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ServiceInstances(testnamespace.Name).Delete(instanceNameK8sNamesDef, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the instance with k8s name and default plan")

		By("Waiting for ServiceInstance with k8s name and default plan to not exist")
		err = util.WaitForInstanceToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), testnamespace.Name, instanceNameK8sNamesDef)
		Expect(err).NotTo(HaveOccurred())

		By("Deleting the test namespace")
		err = framework.DeleteKubeNamespace(f.KubeClientSet, testnamespace.Name)
		Expect(err).NotTo(HaveOccurred())

		// Deleting ClusterServiceBroker and ClusterServiceClass
		By("Deleting the ClusterServiceBroker")
		err = f.ServiceCatalogClientSet.ServicecatalogV1beta1().ClusterServiceBrokers().Delete(brokerName, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the broker")

		By("Waiting for ClusterServiceBroker to not exist")
		err = util.WaitForBrokerToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), brokerName)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for ClusterServiceClass to not exist")
		err = util.WaitForClusterServiceClassToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1beta1(), serviceclassID)
		Expect(err).NotTo(HaveOccurred())
	})
})
