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

package brokerapi

// Service represents a service (of which there may be many variants-- "plans")
// offered by a service broker
//
// https://github.com/openservicebrokerapi/servicebroker/blob/v2.12/spec.md#service-objects
type Service struct {
	Name            string        `json:"name"`
	ID              string        `json:"id"`
	Description     string        `json:"description"`
	Tags            []string      `json:"tags,omitempty"`
	Requires        []string      `json:"requires,omitempty"`
	Bindable        bool          `json:"bindable"`
	Metadata        interface{}   `json:"metadata,omitempty"`
	DashboardClient interface{}   `json:"dashboard_client"`
	PlanUpdateable  bool          `json:"plan_updateable,omitempty"`
	Plans           []ServicePlan `json:"plans"`
}
