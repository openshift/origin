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

package e2e

import (
	v1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"github.com/kubernetes-incubator/service-catalog/test/e2e/framework"
	"github.com/kubernetes-incubator/service-catalog/test/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func newTestBroker(name, url string) *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.BrokerSpec{
			URL: url,
		},
	}
}

var _ = framework.ServiceCatalogDescribe("Broker", func() {
	f := framework.NewDefaultFramework("create-broker")

	brokerName := "test-broker"

	BeforeEach(func() {
		By("Creating a user broker pod")
		pod, err := f.KubeClientSet.CoreV1().Pods(f.Namespace.Name).Create(NewUPSBrokerPod(brokerName))
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for pod to be running")
		err = framework.WaitForPodRunningInNamespace(f.KubeClientSet, pod)
		Expect(err).NotTo(HaveOccurred())
		By("Creating a user broker service")
		_, err = f.KubeClientSet.CoreV1().Services(f.Namespace.Name).Create(NewUPSBrokerService(brokerName))
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		By("Deleting the user broker pod")
		err := f.KubeClientSet.CoreV1().Pods(f.Namespace.Name).Delete(brokerName, nil)
		Expect(err).NotTo(HaveOccurred())
		By("Deleting the user broker service")
		err = f.KubeClientSet.CoreV1().Services(f.Namespace.Name).Delete(brokerName, nil)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should become ready", func() {
		By("Creating a Broker")

		url := "http://test-broker." + f.Namespace.Name + ".svc.cluster.local"
		broker, err := f.ServiceCatalogClientSet.ServicecatalogV1alpha1().Brokers().Create(newTestBroker(brokerName, url))
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for Broker to be ready")
		err = util.WaitForBrokerCondition(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(),
			broker.Name,
			v1alpha1.BrokerCondition{
				Type:   v1alpha1.BrokerConditionReady,
				Status: v1alpha1.ConditionTrue,
			})
		Expect(err).NotTo(HaveOccurred())

		By("Deleting the Broker")
		err = f.ServiceCatalogClientSet.ServicecatalogV1alpha1().Brokers().Delete(brokerName, nil)
		Expect(err).NotTo(HaveOccurred())

		By("Waiting for Broker to not exist")
		err = util.WaitForBrokerToNotExist(f.ServiceCatalogClientSet.ServicecatalogV1alpha1(), brokerName)
		Expect(err).NotTo(HaveOccurred())
	})
})
