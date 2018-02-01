package brokerapi_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/brokerapi"
)

var _ = Describe("Catalog", func() {
	Describe("Service", func() {
		Describe("JSON encoding", func() {
			It("uses the correct keys", func() {
				service := brokerapi.Service{
					ID:            "ID-1",
					Name:          "Cassandra",
					Description:   "A Cassandra Plan",
					Bindable:      true,
					Plans:         []brokerapi.ServicePlan{},
					Metadata:      &brokerapi.ServiceMetadata{},
					Tags:          []string{"test"},
					PlanUpdatable: true,
					DashboardClient: &brokerapi.ServiceDashboardClient{
						ID:          "Dashboard ID",
						Secret:      "dashboardsecret",
						RedirectURI: "the.dashboa.rd",
					},
				}
				jsonString := `{
					"id":"ID-1",
				  	"name":"Cassandra",
					"description":"A Cassandra Plan",
					"bindable":true,
					"plan_updateable":true,
					"tags":["test"],
					"plans":[],
					"dashboard_client":{
						"id":"Dashboard ID",
						"secret":"dashboardsecret",
						"redirect_uri":"the.dashboa.rd"
					},
					"metadata":{

					}
				}`
				Expect(json.Marshal(service)).To(MatchJSON(jsonString))
			})
		})

		It("encodes the optional 'requires' fields", func() {
			service := brokerapi.Service{
				ID:            "ID-1",
				Name:          "Cassandra",
				Description:   "A Cassandra Plan",
				Bindable:      true,
				Plans:         []brokerapi.ServicePlan{},
				Metadata:      &brokerapi.ServiceMetadata{},
				Tags:          []string{"test"},
				PlanUpdatable: true,
				Requires: []brokerapi.RequiredPermission{
					brokerapi.PermissionRouteForwarding,
					brokerapi.PermissionSyslogDrain,
					brokerapi.PermissionVolumeMount,
				},
				DashboardClient: &brokerapi.ServiceDashboardClient{
					ID:          "Dashboard ID",
					Secret:      "dashboardsecret",
					RedirectURI: "the.dashboa.rd",
				},
			}
			jsonString := `{
				"id":"ID-1",
					"name":"Cassandra",
				"description":"A Cassandra Plan",
				"bindable":true,
				"plan_updateable":true,
				"tags":["test"],
				"plans":[],
				"requires": ["route_forwarding", "syslog_drain", "volume_mount"],
				"dashboard_client":{
					"id":"Dashboard ID",
					"secret":"dashboardsecret",
					"redirect_uri":"the.dashboa.rd"
				},
				"metadata":{

				}
			}`
			Expect(json.Marshal(service)).To(MatchJSON(jsonString))
		})
	})

	Describe("ServicePlan", func() {
		Describe("JSON encoding", func() {
			It("uses the correct keys", func() {
				plan := brokerapi.ServicePlan{
					ID:          "ID-1",
					Name:        "Cassandra",
					Description: "A Cassandra Plan",
					Bindable:    brokerapi.BindableValue(true),
					Free:        brokerapi.FreeValue(true),
					Metadata: &brokerapi.ServicePlanMetadata{
						Bullets:     []string{"hello", "its me"},
						DisplayName: "name",
					},
				}
				jsonString := `{
					"id":"ID-1",
					"name":"Cassandra",
					"description":"A Cassandra Plan",
					"free": true,
					"bindable": true,
					"metadata":{
						"bullets":["hello", "its me"],
						"displayName":"name"
					}
				}`

				Expect(json.Marshal(plan)).To(MatchJSON(jsonString))
			})
		})
	})

	Describe("ServicePlanMetadata", func() {
		Describe("JSON encoding", func() {
			It("uses the correct keys", func() {
				metadata := brokerapi.ServicePlanMetadata{
					Bullets:     []string{"test"},
					DisplayName: "Some display name",
				}
				jsonString := `{"bullets":["test"],"displayName":"Some display name"}`

				Expect(json.Marshal(metadata)).To(MatchJSON(jsonString))
			})
		})
	})

	Describe("ServiceMetadata", func() {
		Describe("JSON encoding", func() {
			It("uses the correct keys", func() {
				shareable := true
				metadata := brokerapi.ServiceMetadata{
					DisplayName:         "Cassandra",
					LongDescription:     "A long description of Cassandra",
					DocumentationUrl:    "doc",
					SupportUrl:          "support",
					ImageUrl:            "image",
					ProviderDisplayName: "display",
					Shareable:           &shareable,
				}
				jsonString := `{
					"displayName":"Cassandra",
					"longDescription":"A long description of Cassandra",
					"documentationUrl":"doc",
					"supportUrl":"support",
					"imageUrl":"image",
					"providerDisplayName":"display",
					"shareable":true
				}`

				Expect(json.Marshal(metadata)).To(MatchJSON(jsonString))
			})
		})
	})
})
