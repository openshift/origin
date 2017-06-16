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

package server

import (
	"net/http/httptest"

	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi/openservicebroker"
)

// NewCreateFunc creates a new brokerapi.CreateFunc according to a broker server running
// in srv
func NewCreateFunc(srv *httptest.Server, user, pass string) brokerapi.CreateFunc {
	// type CreateFunc func(name, url, username, password string) BrokerClient
	return brokerapi.CreateFunc(func(name, url, username, password string) brokerapi.BrokerClient {
		return openservicebroker.NewClient("testclient", srv.URL, user, pass)
	})
}
