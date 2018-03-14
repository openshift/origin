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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"

	. "github.com/kubernetes-incubator/service-catalog/pkg/svcat/service-catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Broker", func() {
	var (
		sdk          *SDK
		svcCatClient *fake.Clientset
		sb           *v1beta1.ClusterServiceBroker
		sb2          *v1beta1.ClusterServiceBroker
	)

	BeforeEach(func() {
		sb = &v1beta1.ClusterServiceBroker{ObjectMeta: metav1.ObjectMeta{Name: "foobar"}}
		sb2 = &v1beta1.ClusterServiceBroker{ObjectMeta: metav1.ObjectMeta{Name: "barbaz"}}
		svcCatClient = fake.NewSimpleClientset(sb, sb2)
		sdk = &SDK{
			ServiceCatalogClient: svcCatClient,
		}
	})

	Describe("RetrieveBrokers", func() {
		It("Calls the generated v1beta1 List method", func() {
			brokers, err := sdk.RetrieveBrokers()

			Expect(err).NotTo(HaveOccurred())
			Expect(brokers).Should(ConsistOf(*sb, *sb2))
			Expect(svcCatClient.Actions()[0].Matches("list", "clusterservicebrokers")).To(BeTrue())
		})
		It("Bubbles up errors", func() {
			badClient := &fake.Clientset{}
			errorMessage := "error retrieving list"
			badClient.AddReactor("list", "clusterservicebrokers", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient
			_, err := sdk.RetrieveBrokers()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errorMessage))
			Expect(badClient.Actions()[0].Matches("list", "clusterservicebrokers")).To(BeTrue())
		})
	})
	Describe("RetrieveBroker", func() {
		It("Calls the generated v1beta1 List method with the passed in broker", func() {
			broker, err := sdk.RetrieveBroker(sb.Name)

			Expect(err).NotTo(HaveOccurred())
			Expect(broker).To(Equal(sb))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "clusterservicebrokers")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(sb.Name))
		})
		It("Bubbles up errors", func() {
			brokerName := "banana"

			broker, err := sdk.RetrieveBroker(brokerName)

			Expect(broker).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "clusterservicebrokers")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(brokerName))
		})
	})
	Describe("RetrieveBrokerByClass", func() {
		It("Calls the generated v1beta1 List method with the passed in class's parent broker", func() {
			sc := &v1beta1.ClusterServiceClass{Spec: v1beta1.ClusterServiceClassSpec{ClusterServiceBrokerName: sb.Name}}
			broker, err := sdk.RetrieveBrokerByClass(sc)

			Expect(broker).NotTo(BeNil())
			Expect(err).NotTo(HaveOccurred())
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "clusterservicebrokers")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(sb.Name))
		})

		It("Bubbles up errors", func() {
			brokerName := "banana"
			sc := &v1beta1.ClusterServiceClass{Spec: v1beta1.ClusterServiceClassSpec{ClusterServiceBrokerName: brokerName}}
			broker, err := sdk.RetrieveBrokerByClass(sc)

			Expect(broker).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			actions := svcCatClient.Actions()
			Expect(actions[0].Matches("get", "clusterservicebrokers")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(brokerName))
		})
	})
	Describe("Sync", func() {
		It("Useds the generated b1beta1 Retrieve method to get the broker, and then updates it with a new RelistRequests", func() {
			err := sdk.Sync(sb.Name, 3)
			Expect(err).NotTo(HaveOccurred())

			actions := svcCatClient.Actions()
			Expect(len(actions) >= 2).To(BeTrue())
			Expect(actions[0].Matches("get", "clusterservicebrokers")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(sb.Name))

			Expect(actions[1].Matches("update", "clusterservicebrokers")).To(BeTrue())
			Expect(actions[1].(testing.UpdateActionImpl).Object.(*v1beta1.ClusterServiceBroker).Spec.RelistRequests).Should(BeNumerically(">", 0))
		})
	})
})
