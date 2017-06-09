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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/pkg/api/v1"

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

func newTestBrokerPod(name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  name,
					Image: "quay.io/kubernetes-service-catalog/user-broker:v0.0.7",
					Args: []string{
						"--port",
						"8080",
					},
					Ports: []v1.ContainerPort{
						{
							ContainerPort: 8080,
						},
					},
				},
			},
		},
	}
}

func newTestBrokerService(name string) *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"app": name,
			},
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

var _ = framework.ServiceCatalogDescribe("Broker", func() {
	f := framework.NewDefaultFramework("create-broker")

	brokerName := "test-broker"

	BeforeEach(func() {
		By("Creating a user broker pod")
		pod, err := f.KubeClientSet.CoreV1().Pods(f.Namespace.Name).Create(newTestBrokerPod(brokerName))
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for pod to be running")
		err = framework.WaitForPodRunningInNamespace(f.KubeClientSet, pod)
		Expect(err).NotTo(HaveOccurred())
		By("Creating a user broker service")
		_, err = f.KubeClientSet.CoreV1().Services(f.Namespace.Name).Create(newTestBrokerService(brokerName))
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
		f.ServiceCatalogClientSet.ServicecatalogV1alpha1().Brokers().Delete(brokerName, nil)
	})
})
