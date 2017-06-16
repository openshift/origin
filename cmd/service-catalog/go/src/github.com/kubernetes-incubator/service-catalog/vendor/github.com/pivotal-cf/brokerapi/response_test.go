package brokerapi_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
)

var _ = Describe("Catalog Response", func() {
	Describe("JSON encoding", func() {
		It("has a list of services", func() {
			catalogResponse := brokerapi.CatalogResponse{
				Services: []brokerapi.Service{},
			}
			jsonString := `{"services":[]}`

			Expect(json.Marshal(catalogResponse)).To(MatchJSON(jsonString))
		})
	})
})

var _ = Describe("Provisioning Response", func() {
	Describe("JSON encoding", func() {
		Context("when the dashboard URL is not present", func() {
			It("does not return it in the JSON", func() {
				provisioningResponse := brokerapi.ProvisioningResponse{}
				jsonString := `{}`

				Expect(json.Marshal(provisioningResponse)).To(MatchJSON(jsonString))
			})
		})

		Context("when the dashboard URL is present", func() {
			It("returns it in the JSON", func() {
				provisioningResponse := brokerapi.ProvisioningResponse{
					DashboardURL: "http://example.com/broker",
				}
				jsonString := `{"dashboard_url":"http://example.com/broker"}`

				Expect(json.Marshal(provisioningResponse)).To(MatchJSON(jsonString))
			})
		})
	})
})

var _ = Describe("Binding Response", func() {
	Describe("JSON encoding", func() {
		It("has a credentials object", func() {
			binding := brokerapi.Binding{}
			jsonString := `{"credentials":null}`

			Expect(json.Marshal(binding)).To(MatchJSON(jsonString))
		})
	})
})

var _ = Describe("Error Response", func() {
	Describe("JSON encoding", func() {
		It("has a description field", func() {
			errorResponse := brokerapi.ErrorResponse{
				Description: "a bad thing happened",
			}
			jsonString := `{"description":"a bad thing happened"}`

			Expect(json.Marshal(errorResponse)).To(MatchJSON(jsonString))
		})
	})
})
