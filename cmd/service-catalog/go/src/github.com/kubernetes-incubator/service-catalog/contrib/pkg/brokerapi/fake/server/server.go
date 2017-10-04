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

	"github.com/pivotal-cf/brokerapi"
)

// Run runs a new test server from the given broker handler and auth credentials
func Run(hdl *Handler, username, password string) *httptest.Server {
	httpHandler := brokerapi.New(hdl, logger, brokerapi.BrokerCredentials{
		Username: username,
		Password: password,
	})

	srv := httptest.NewServer(httpHandler)
	return srv
}
