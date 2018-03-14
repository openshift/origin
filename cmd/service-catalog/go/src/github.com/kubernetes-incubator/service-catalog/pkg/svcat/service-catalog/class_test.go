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
	"errors"
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

var _ = Describe("Class", func() {
	var (
		sdk          *SDK
		svcCatClient *fake.Clientset
		sc           *v1beta1.ClusterServiceClass
		sc2          *v1beta1.ClusterServiceClass
	)

	BeforeEach(func() {
		sc = &v1beta1.ClusterServiceClass{ObjectMeta: metav1.ObjectMeta{Name: "foobar"}}
		sc2 = &v1beta1.ClusterServiceClass{ObjectMeta: metav1.ObjectMeta{Name: "barbaz"}}
		svcCatClient = fake.NewSimpleClientset(sc, sc2)
		sdk = &SDK{
			ServiceCatalogClient: svcCatClient,
		}
	})

	Describe("RetrieveClasses", func() {
		It("Calls the generated v1beta1 List method", func() {
			classes, err := sdk.RetrieveClasses()

			Expect(err).NotTo(HaveOccurred())
			Expect(classes).Should(ConsistOf(*sc, *sc2))
			Expect(svcCatClient.Actions()[0].Matches("list", "clusterserviceclasses")).To(BeTrue())
		})
		It("Bubbles up errors", func() {
			badClient := &fake.Clientset{}
			errorMessage := "error retrieving list"
			badClient.AddReactor("list", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf(errorMessage)
			})
			sdk = &SDK{
				ServiceCatalogClient: badClient,
			}

			_, err := sdk.RetrieveClasses()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(errorMessage))
			Expect(badClient.Actions()[0].Matches("list", "clusterserviceclasses")).To(BeTrue())
		})
	})
	Describe("RetrieveClassByName", func() {
		It("Calls the generated v1beta1 List method with the passed in class", func() {
			className := sc.Name
			realClient := &fake.Clientset{}
			realClient.AddReactor("list", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &v1beta1.ClusterServiceClassList{Items: []v1beta1.ClusterServiceClass{*sc}}, nil
			})
			sdk = &SDK{
				ServiceCatalogClient: realClient,
			}
			class, err := sdk.RetrieveClassByName(className)

			Expect(err).NotTo(HaveOccurred())
			Expect(class).To(Equal(sc))
			actions := realClient.Actions()
			Expect(actions[0].Matches("list", "clusterserviceclasses")).To(BeTrue())
			requirements := actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Requirements()
			Expect(requirements).ShouldNot(BeEmpty())
			Expect(requirements[0].Field).To(Equal("spec.externalName"))
			Expect(requirements[0].Value).To(Equal(className))
		})
		It("Bubbles up errors", func() {
			className := "notreal_class"
			emptyClient := &fake.Clientset{}
			emptyClient.AddReactor("list", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &v1beta1.ClusterServiceClassList{Items: []v1beta1.ClusterServiceClass{}}, nil
			})
			sdk = &SDK{
				ServiceCatalogClient: emptyClient,
			}
			class, err := sdk.RetrieveClassByName(className)

			Expect(class).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			actions := emptyClient.Actions()
			Expect(actions[0].Matches("list", "clusterserviceclasses")).To(BeTrue())
			requirements := actions[0].(testing.ListActionImpl).GetListRestrictions().Fields.Requirements()
			Expect(requirements).ShouldNot(BeEmpty())
			Expect(requirements[0].Field).To(Equal("spec.externalName"))
			Expect(requirements[0].Value).To(Equal(className))
		})
	})
	Describe("RetrieveClassByID", func() {
		It("Calls the generated v1beta1 get method", func() {
			classID := fmt.Sprintf("%v", sc.UID)
			realClient := &fake.Clientset{}
			realClient.AddReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, sc, nil
			})
			sdk = &SDK{
				ServiceCatalogClient: realClient,
			}
			class, err := sdk.RetrieveClassByID(classID)
			Expect(err).NotTo(HaveOccurred())
			Expect(fmt.Sprintf("%v", class.UID)).To(Equal(classID))
			actions := realClient.Actions()
			Expect(actions[0].Matches("get", "clusterserviceclasses")).To(BeTrue())
		})
		It("Bubbles up errors", func() {
			errorMessage := "not found"
			emptyClient := &fake.Clientset{}
			emptyClient.AddReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New(errorMessage)
			})
			sdk = &SDK{
				ServiceCatalogClient: emptyClient,
			}
			class, err := sdk.RetrieveClassByID("not_real")

			Expect(class).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("not found"))
			actions := emptyClient.Actions()
			Expect(actions[0].Matches("get", "clusterserviceclasses")).To(BeTrue())
		})
	})
	Describe("RetrieveClassByPlan", func() {
		It("Calls the generated v1beta1 get method with the plan's parent service class's name", func() {
			classPlan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{
					ClusterServiceClassRef: v1beta1.ClusterObjectReference{
						Name: sc.Name,
					},
				},
			}
			realClient := &fake.Clientset{}
			realClient.AddReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, sc, nil
			})
			sdk = &SDK{
				ServiceCatalogClient: realClient,
			}
			class, err := sdk.RetrieveClassByPlan(classPlan)
			Expect(err).NotTo(HaveOccurred())
			Expect(class).To(Equal(sc))
			actions := realClient.Actions()
			Expect(actions[0].Matches("get", "clusterserviceclasses")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(sc.Name))
		})
		It("Bubbles up errors", func() {
			fakeClassName := "not_real"
			errorMessage := "not found"

			classPlan := &v1beta1.ClusterServicePlan{
				ObjectMeta: metav1.ObjectMeta{
					Name: "foobar_plan",
				},
				Spec: v1beta1.ClusterServicePlanSpec{
					ClusterServiceClassRef: v1beta1.ClusterObjectReference{
						Name: fakeClassName,
					},
				},
			}
			badClient := &fake.Clientset{}
			badClient.AddReactor("get", "clusterserviceclasses", func(action testing.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New(errorMessage)
			})
			sdk = &SDK{
				ServiceCatalogClient: badClient,
			}
			class, err := sdk.RetrieveClassByPlan(classPlan)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(errorMessage))
			Expect(class).To(BeNil())
			actions := badClient.Actions()
			Expect(actions[0].Matches("get", "clusterserviceclasses")).To(BeTrue())
			Expect(actions[0].(testing.GetActionImpl).Name).To(Equal(fakeClassName))
		})
	})
})
