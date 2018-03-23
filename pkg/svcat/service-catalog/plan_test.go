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
	"k8s.io/client-go/testing"

	. "github.com/kubernetes-incubator/service-catalog/pkg/svcat/service-catalog"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Plan", func() {
	var (
		sdk          *SDK
		svcCatClient *fake.Clientset
		sp           *v1beta1.ClusterServicePlan
		sp2          *v1beta1.ClusterServicePlan
	)

	BeforeEach(func() {
		sp = &v1beta1.ClusterServicePlan{ObjectMeta: metav1.ObjectMeta{Name: "foobar"}}
		sp2 = &v1beta1.ClusterServicePlan{ObjectMeta: metav1.ObjectMeta{Name: "barbaz"}}
		svcCatClient = fake.NewSimpleClientset(sp, sp2)
		sdk = &SDK{
			ServiceCatalogClient: svcCatClient,
		}
	})

	Describe("RetrivePlans", func() {
		It("Calls the generated v1beta1 List method", func() {
			plans, err := sdk.RetrievePlans(nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(plans).Should(ConsistOf(*sp, *sp2))
			Expect(svcCatClient.Actions()[0].Matches("list", "clusterserviceplans")).To(BeTrue())
		})
		It("Bubbles up errors", func() {
			errorMessage := "error retrieving list"
			badClient := &fake.Clientset{}
			badClient.AddReactor("list", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient
			_, err := sdk.RetrievePlans(nil)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errorMessage))
			Expect(badClient.Actions()[0].Matches("list", "clusterserviceplans")).To(BeTrue())
		})
	})
	Describe("RetrievePlanByName", func() {
		It("Calls the generated v1beta1 List method with the passed in plan name", func() {
			planName := sp.Name
			singleClient := &fake.Clientset{}
			singleClient.AddReactor("list", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &v1beta1.ClusterServicePlanList{Items: []v1beta1.ClusterServicePlan{*sp}}, nil
			})
			sdk.ServiceCatalogClient = singleClient

			plan, err := sdk.RetrievePlanByName(planName)

			Expect(err).NotTo(HaveOccurred())
			Expect(plan.Name).To(Equal(planName))
			actions := singleClient.Actions()
			Expect(len(actions)).To(Equal(1))
			Expect(actions[0].Matches("list", "clusterserviceplans")).To(BeTrue())
			opts := fields.Set{"spec.externalName": planName}
			Expect(actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Matches(opts)).To(BeTrue())
		})
		It("Bubbles up errors", func() {
			planName := "not_real"
			errorMessage := "plan not found"
			badClient := &fake.Clientset{}
			badClient.AddReactor("list", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			plan, err := sdk.RetrievePlanByName(planName)

			Expect(plan).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errorMessage))
			actions := badClient.Actions()
			Expect(len(actions)).To(Equal(1))
			Expect(actions[0].Matches("list", "clusterserviceplans")).To(BeTrue())
			opts := fields.Set{"spec.externalName": planName}
			Expect(actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Matches(opts)).To(BeTrue())
		})
	})
	Describe("RetrievePlanByID", func() {
		It("Calls the generated v1beta1 get method with the passed in uuid", func() {
			planID := sp.Name
			_, err := sdk.RetrievePlanByID(planID)
			Expect(err).NotTo(HaveOccurred())
			actions := svcCatClient.Actions()
			Expect(len(actions)).To(Equal(1))
			Expect(actions[0].Matches("get", "clusterserviceplans")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(planID))
		})
		It("Bubbles up errors", func() {
			planID := "not_real"
			errorMessage := "plan not found"
			badClient := &fake.Clientset{}
			badClient.AddReactor("get", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			plan, err := sdk.RetrievePlanByID(planID)

			Expect(plan).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errorMessage))
			actions := badClient.Actions()
			Expect(len(actions)).To(Equal(1))
			Expect(actions[0].Matches("get", "clusterserviceplans")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(planID))
		})
	})
	Describe("RetrievePlansByClass", func() {
		It("Calls the generated v1beta1 List  method with an opts containing the passed in class' name", func() {
			class := &v1beta1.ClusterServiceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "durian_class",
				},
				Spec: v1beta1.ClusterServiceClassSpec{},
			}
			plan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "durian",
				},
				Spec: v1beta1.ClusterServicePlanSpec{
					ClusterServiceClassRef: v1beta1.ClusterObjectReference{
						Name: class.Name,
					},
				},
			}
			linkedClient := fake.NewSimpleClientset(class, plan)
			linkedClient.AddReactor("list", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &v1beta1.ClusterServicePlanList{Items: []v1beta1.ClusterServicePlan{*plan}}, nil
			})
			sdk.ServiceCatalogClient = linkedClient
			retPlans, err := sdk.RetrievePlansByClass(class)
			Expect(retPlans).To(ConsistOf(*plan))
			Expect(err).NotTo(HaveOccurred())
			actions := linkedClient.Actions()
			Expect(len(actions)).To(Equal(1))
			Expect(actions[0].Matches("list", "clusterserviceplans")).To(BeTrue())
			opts := fields.Set{"spec.clusterServiceClassRef.name": class.Name}
			Expect(actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Matches(opts)).To(BeTrue())
		})
		It("Bubbles up errors", func() {
			errorMessage := "no plans found"
			class := &v1beta1.ClusterServiceClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "durian_class",
				},
				Spec: v1beta1.ClusterServiceClassSpec{},
			}
			badClient := &fake.Clientset{}
			badClient.AddReactor("list", "clusterserviceplans", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk.ServiceCatalogClient = badClient

			plans, err := sdk.RetrievePlansByClass(class)
			Expect(plans).To(BeNil())
			Expect(err).To(HaveOccurred())
			actions := badClient.Actions()
			Expect(len(actions)).To(Equal(1))
			Expect(actions[0].Matches("list", "clusterserviceplans")).To(BeTrue())
			opts := fields.Set{"spec.clusterServiceClassRef.name": class.Name}
			Expect(actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Matches(opts)).To(BeTrue())
		})
	})
})
