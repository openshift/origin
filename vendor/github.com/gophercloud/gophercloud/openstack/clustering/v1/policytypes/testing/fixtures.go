package testing

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/gophercloud/gophercloud/openstack/clustering/v1/policytypes"
	th "github.com/gophercloud/gophercloud/testhelper"
	fake "github.com/gophercloud/gophercloud/testhelper/client"
)

const PolicyTypeBody = `
{
	"policy_types": [
		{
			"name": "senlin.policy.affinity",
			"version": "1.0",
			"support_status": {
				"1.0": [
					{
						"status": "SUPPORTED",
						"since": "2016.10"
					}
				]
			}
		},
		{
			"name": "senlin.policy.health",
			"version": "1.0",
			"support_status": {
				"1.0": [
					{
						"status": "EXPERIMENTAL",
						"since": "2016.10"
					}
				]
			}
		},
		{
			"name": "senlin.policy.scaling",
			"version": "1.0",
			"support_status": {
				"1.0": [
					{
						"status": "SUPPORTED",
						"since": "2016.04"
					}
				]
			}
		},
		{
			"name": "senlin.policy.region_placement",
			"version": "1.0",
			"support_status": {
				"1.0": [
					{
						"status": "EXPERIMENTAL",
						"since": "2016.04"
					},
					{
						"status": "SUPPORTED",
						"since": "2016.10"
					}
				]
			}
		}
	]
}
`

var (
	ExpectedPolicyTypes = []policytypes.PolicyType{
		{
			Name:    "senlin.policy.affinity",
			Version: "1.0",
			SupportStatus: map[string][]policytypes.SupportStatusType{
				"1.0": {
					{
						Status: "SUPPORTED",
						Since:  "2016.10",
					},
				},
			},
		},
		{
			Name:    "senlin.policy.health",
			Version: "1.0",
			SupportStatus: map[string][]policytypes.SupportStatusType{
				"1.0": {
					{
						Status: "EXPERIMENTAL",
						Since:  "2016.10",
					},
				},
			},
		},
		{
			Name:    "senlin.policy.scaling",
			Version: "1.0",
			SupportStatus: map[string][]policytypes.SupportStatusType{
				"1.0": {
					{
						Status: "SUPPORTED",
						Since:  "2016.04",
					},
				},
			},
		},
		{
			Name:    "senlin.policy.region_placement",
			Version: "1.0",
			SupportStatus: map[string][]policytypes.SupportStatusType{
				"1.0": {
					{
						Status: "EXPERIMENTAL",
						Since:  "2016.04",
					},
					{
						Status: "SUPPORTED",
						Since:  "2016.10",
					},
				},
			},
		},
	}
)

func HandlePolicyTypeList(t *testing.T) {
	th.Mux.HandleFunc("/v1/policy-types", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, PolicyTypeBody)
	})
}
