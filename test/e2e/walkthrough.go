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
	v1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/test/e2e/framework"
	"github.com/kubernetes-incubator/service-catalog/test/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"

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
			brokerName       = upsbrokername
			serviceclassName = "user-provided-service"
			testns           = "test-ns"
			instanceName     = "ups-instance"
			bindingName      = "ups-instance-credential"
		)

		// Broker and ServiceClass should become ready
		By("Make sure the named ServiceBroker does not exist before create")
		if _, err := f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceBrokers().Get(brokerName, metav1.GetOptions{}); err == nil {
			err = f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceBrokers().Delete(brokerName, nil)
			Expect(err).NotTo(HaveOccurred(), "failed to delete the broker")

			By("Waiting for ServiceBroker to not exist")
			err = util.WaitForBrokerToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(), brokerName)
			Expect(err).NotTo(HaveOccurred())
		}

		By("Creating a ServiceBroker")
		url := "http://" + upsbrokername + "." + f.Namespace.Name + ".svc.cluster.local"
		broker := &v1alpha1.ServiceBroker{
			ObjectMeta: metav1.ObjectMeta{
				Name: brokerName,
			},
			Spec: v1alpha1.ServiceBrokerSpec{
				URL: url,
			},
		}
		broker, err := f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceBrokers().Create(broker)
		Expect(err).NotTo(HaveOccurred(), "failed to create ServiceBroker")

		By("Waiting for ServiceBroker to be ready")
		err = util.WaitForBrokerCondition(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(),
			broker.Name,
			v1alpha1.ServiceBrokerCondition{
				Type:   v1alpha1.ServiceBrokerConditionReady,
				Status: v1alpha1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait ServiceBroker to be ready")

		By("Waiting for ServiceClass to be ready")
		err = util.WaitForServiceClassToExist(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(), serviceclassName)
		Expect(err).NotTo(HaveOccurred(), "failed to wait serviceclass to be ready")

		// Provisioning a ServiceInstance and binding to it
		By("Creating a namespace")
		testnamespace, err := framework.CreateKubeNamespace(testns, f.KubeClientSet)
		Expect(err).NotTo(HaveOccurred(), "failed to create kube namespace")

		By("Creating a ServiceInstance")
		instance := &v1alpha1.ServiceInstance{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceName,
				Namespace: testnamespace.Name,
			},
			Spec: v1alpha1.ServiceInstanceSpec{
				ServiceClassName: serviceclassName,
				PlanName:         "default",
			},
		}
		instance, err = f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceInstances(testnamespace.Name).Create(instance)
		Expect(err).NotTo(HaveOccurred(), "failed to create instance")
		Expect(instance).NotTo(BeNil())

		By("Waiting for ServiceInstance to be ready")
		err = util.WaitForInstanceCondition(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(),
			testnamespace.Name,
			instanceName,
			v1alpha1.ServiceInstanceCondition{
				Type:   v1alpha1.ServiceInstanceConditionReady,
				Status: v1alpha1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait instance to be ready")

		// Binding to the ServiceInstance
		By("Creating a ServiceInstanceCredential")
		binding := &v1alpha1.ServiceInstanceCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name:      bindingName,
				Namespace: testnamespace.Name,
			},
			Spec: v1alpha1.ServiceInstanceCredentialSpec{
				ServiceInstanceRef: v1.LocalObjectReference{
					Name: instanceName,
				},
				SecretName: "my-secret",
			},
		}
		binding, err = f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceInstanceCredentials(testnamespace.Name).Create(binding)
		Expect(err).NotTo(HaveOccurred(), "failed to create binding")
		Expect(binding).NotTo(BeNil())

		By("Waiting for ServiceInstanceCredential to be ready")
		err = util.WaitForBindingCondition(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(),
			testnamespace.Name,
			bindingName,
			v1alpha1.ServiceInstanceCredentialCondition{
				Type:   v1alpha1.ServiceInstanceCredentialConditionReady,
				Status: v1alpha1.ConditionTrue,
			},
		)
		Expect(err).NotTo(HaveOccurred(), "failed to wait binding to be ready")

		By("Secret should have been created after binding")
		_, err = f.KubeClientSet.CoreV1().Secrets(testnamespace.Name).Get("my-secret", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create secret after binding")

		// Unbinding from the ServiceInstance
		By("Deleting the ServiceInstanceCredential")
		err = f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceInstanceCredentials(testnamespace.Name).Delete(bindingName, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the binding")

		By("Waiting for ServiceInstanceCredential to not exist")
		err = util.WaitForBindingToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(), testnamespace.Name, bindingName)
		Expect(err).NotTo(HaveOccurred())

		By("Secret should been deleted after delete the binding")
		_, err = f.KubeClientSet.CoreV1().Secrets(testnamespace.Name).Get("my-secret", metav1.GetOptions{})
		Expect(err).To(HaveOccurred())

		// Deprovisioning the ServiceInstance
		By("Deleting the ServiceInstance")
		err = f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceInstances(testnamespace.Name).Delete(instanceName, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the instance")

		By("Waiting for ServiceInstance to not exist")
		err = util.WaitForInstanceToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(), testnamespace.Name, instanceName)
		Expect(err).NotTo(HaveOccurred())

		By("Deleting the test namespace")
		err = framework.DeleteKubeNamespace(f.KubeClientSet, testnamespace.Name)
		Expect(err).NotTo(HaveOccurred())

		// Deleting ServiceBroker and ServiceClass
		By("Deleting the ServiceBroker")
		err = f.ServiceCatalogClientSet.ServicecatalogV1alpha1().ServiceBrokers().Delete(brokerName, nil)
		Expect(err).NotTo(HaveOccurred(), "failed to delete the broker")

		By("Waiting for ServiceBroker to not exist")
		err = util.WaitForBrokerToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(), brokerName)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for ServiceClass to not exist")
		err = util.WaitForServiceClassToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(), serviceclassName)
		Expect(err).NotTo(HaveOccurred())
	})
})
