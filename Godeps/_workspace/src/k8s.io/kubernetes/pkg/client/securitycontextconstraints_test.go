/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package client

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"net/url"
)

func TestSecurityContextConstraintsCreate(t *testing.T) {
	ns := api.NamespaceNone
	scc := &api.SecurityContextConstraints{
		ObjectMeta: api.ObjectMeta{
			Name: "abc",
		},
	}

	c := &testClient{
		Request: testRequest{
			Method: "POST",
			Path:   testapi.ResourcePath(getSCCResoureName(), ns, ""),
			Query:  buildQueryValues(nil),
			Body:   scc,
		},
		Response: Response{StatusCode: 200, Body: scc},
	}

	response, err := c.Setup().SecurityContextConstraints().Create(scc)
	c.Validate(t, response, err)
}

func TestSecurityContextConstraintsGet(t *testing.T) {
	ns := api.NamespaceNone
	scc := &api.SecurityContextConstraints{
		ObjectMeta: api.ObjectMeta{
			Name: "abc",
		},
	}
	c := &testClient{
		Request: testRequest{
			Method: "GET",
			Path:   testapi.ResourcePath(getSCCResoureName(), ns, "abc"),
			Query:  buildQueryValues(nil),
			Body:   nil,
		},
		Response: Response{StatusCode: 200, Body: scc},
	}

	response, err := c.Setup().SecurityContextConstraints().Get("abc")
	c.Validate(t, response, err)
}

func TestSecurityContextConstraintsList(t *testing.T) {
	ns := api.NamespaceNone
	sccList := &api.SecurityContextConstraintsList{
		Items: []api.SecurityContextConstraints{
			{
				ObjectMeta: api.ObjectMeta{
					Name: "abc",
				},
			},
		},
	}
	c := &testClient{
		Request: testRequest{
			Method: "GET",
			Path:   testapi.ResourcePath(getSCCResoureName(), ns, ""),
			Query:  buildQueryValues(nil),
			Body:   nil,
		},
		Response: Response{StatusCode: 200, Body: sccList},
	}
	response, err := c.Setup().SecurityContextConstraints().List(labels.Everything(), fields.Everything())
	c.Validate(t, response, err)
}

func TestSecurityContextConstraintsUpdate(t *testing.T) {
	ns := api.NamespaceNone
	scc := &api.SecurityContextConstraints{
		ObjectMeta: api.ObjectMeta{
			Name:            "abc",
			ResourceVersion: "1",
		},
	}
	c := &testClient{
		Request:  testRequest{Method: "PUT", Path: testapi.ResourcePath(getSCCResoureName(), ns, "abc"), Query: buildQueryValues(nil)},
		Response: Response{StatusCode: 200, Body: scc},
	}
	response, err := c.Setup().SecurityContextConstraints().Update(scc)
	c.Validate(t, response, err)
}

func TestSecurityContextConstraintsDelete(t *testing.T) {
	ns := api.NamespaceNone
	c := &testClient{
		Request:  testRequest{Method: "DELETE", Path: testapi.ResourcePath(getSCCResoureName(), ns, "foo"), Query: buildQueryValues(nil)},
		Response: Response{StatusCode: 200},
	}
	err := c.Setup().SecurityContextConstraints().Delete("foo")
	c.Validate(t, nil, err)
}

func TestSecurityContextConstraintsWatch(t *testing.T) {
	c := &testClient{
		Request: testRequest{
			Method: "GET",
			Path:   "/api/" + testapi.Version() + "/watch/" + getSCCResoureName(),
			Query:  url.Values{"resourceVersion": []string{}}},
		Response: Response{StatusCode: 200},
	}
	_, err := c.Setup().SecurityContextConstraints().Watch(labels.Everything(), fields.Everything(), "")
	c.Validate(t, nil, err)
}

func getSCCResoureName() string {
	return "securitycontextconstraints"
}
