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

package cmd

import (
	"bytes"
	"net/http"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
)

func TestRunExposeService(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		ns     string
		calls  map[string]string
		input  runtime.Object
		flags  map[string]string
		output runtime.Object
		status int
	}{
		{
			name: "expose-service-from-service-no-selector-defined",
			args: []string{"service", "baz"},
			ns:   "test",
			calls: map[string]string{
				"GET":  "/namespaces/test/services/baz",
				"POST": "/namespaces/test/services",
			},
			input: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "baz", Namespace: "test", ResourceVersion: "12"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{"app": "go"},
				},
			},
			flags: map[string]string{"protocol": "UDP", "port": "14", "name": "foo", "labels": "svc=test"},
			output: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "", Labels: map[string]string{"svc": "test"}},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{
							Protocol:   api.ProtocolUDP,
							Port:       14,
							TargetPort: util.NewIntOrStringFromInt(14),
						},
					},
					Selector: map[string]string{"app": "go"},
				},
			},
			status: 200,
		},
		{
			name: "expose-service-from-service",
			args: []string{"service", "baz"},
			ns:   "test",
			calls: map[string]string{
				"GET":  "/namespaces/test/services/baz",
				"POST": "/namespaces/test/services",
			},
			input: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "baz", Namespace: "test", ResourceVersion: "12"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{"app": "go"},
				},
			},
			flags: map[string]string{"selector": "func=stream", "protocol": "UDP", "port": "14", "name": "foo", "labels": "svc=test"},
			output: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "", Labels: map[string]string{"svc": "test"}},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{
							Protocol:   api.ProtocolUDP,
							Port:       14,
							TargetPort: util.NewIntOrStringFromInt(14),
						},
					},
					Selector: map[string]string{"func": "stream"},
				},
			},
			status: 200,
		},
		{
			name: "no-name-passed-from-the-cli",
			args: []string{"service", "mayor"},
			ns:   "default",
			calls: map[string]string{
				"GET":  "/namespaces/default/services/mayor",
				"POST": "/namespaces/default/services",
			},
			input: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "mayor", Namespace: "default", ResourceVersion: "12"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{"run": "this"},
				},
			},
			// No --name flag specified below. Service will use the rc's name passed via the 'default-name' parameter
			flags: map[string]string{"selector": "run=this", "port": "80", "labels": "runas=amayor"},
			output: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "mayor", Namespace: "", Labels: map[string]string{"runas": "amayor"}},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{
							Protocol:   api.ProtocolTCP,
							Port:       80,
							TargetPort: util.NewIntOrStringFromInt(80),
						},
					},
					Selector: map[string]string{"run": "this"},
				},
			},
			status: 200,
		},
		{
			name: "expose-external-service",
			args: []string{"service", "baz"},
			ns:   "test",
			calls: map[string]string{
				"GET":  "/namespaces/test/services/baz",
				"POST": "/namespaces/test/services",
			},
			input: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "baz", Namespace: "test", ResourceVersion: "12"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{"app": "go"},
				},
			},
			flags: map[string]string{"selector": "func=stream", "protocol": "UDP", "port": "14", "name": "foo", "labels": "svc=test", "create-external-load-balancer": "true"},
			output: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "", Labels: map[string]string{"svc": "test"}},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{
							Protocol:   api.ProtocolUDP,
							Port:       14,
							TargetPort: util.NewIntOrStringFromInt(14),
						},
					},
					Selector: map[string]string{"func": "stream"},
					Type:     api.ServiceTypeLoadBalancer,
				},
			},
			status: 200,
		},
		{
			name: "expose-external-affinity-service",
			args: []string{"service", "baz"},
			ns:   "test",
			calls: map[string]string{
				"GET":  "/namespaces/test/services/baz",
				"POST": "/namespaces/test/services",
			},
			input: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "baz", Namespace: "test", ResourceVersion: "12"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{"app": "go"},
				},
			},
			flags: map[string]string{"selector": "func=stream", "protocol": "UDP", "port": "14", "name": "foo", "labels": "svc=test", "create-external-load-balancer": "true", "session-affinity": "ClientIP"},
			output: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "foo", Namespace: "", Labels: map[string]string{"svc": "test"}},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{
							Protocol:   api.ProtocolUDP,
							Port:       14,
							TargetPort: util.NewIntOrStringFromInt(14),
						},
					},
					Selector:        map[string]string{"func": "stream"},
					Type:            api.ServiceTypeLoadBalancer,
					SessionAffinity: api.ServiceAffinityClientIP,
				},
			},
			status: 200,
		},
		{
			name: "expose-external-service",
			args: []string{"service", "baz"},
			ns:   "test",
			calls: map[string]string{
				"GET":  "/namespaces/test/services/baz",
				"POST": "/namespaces/test/services",
			},
			input: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "baz", Namespace: "test", ResourceVersion: "12"},
				Spec: api.ServiceSpec{
					Selector: map[string]string{},
					Ports:    []api.ServicePort{},
				},
			},
			// Even if we specify --selector, since service/test doesn't need one it will ignore it
			flags: map[string]string{"selector": "svc=fromexternal", "port": "90", "labels": "svc=fromexternal", "name": "frombaz", "generator": "service/test"},
			output: &api.Service{
				ObjectMeta: api.ObjectMeta{Name: "frombaz", Namespace: "", Labels: map[string]string{"svc": "fromexternal"}},
				Spec: api.ServiceSpec{
					Ports: []api.ServicePort{
						{
							Protocol:   api.ProtocolTCP,
							Port:       90,
							TargetPort: util.NewIntOrStringFromInt(90),
						},
					},
				},
			},
			status: 200,
		},
	}

	for _, test := range tests {
		f, tf, codec := NewAPIFactory()
		tf.Printer = &testPrinter{}
		tf.Client = &client.FakeRESTClient{
			Codec: codec,
			Client: client.HTTPClientFunc(func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == test.calls[m] && m == "GET":
					return &http.Response{StatusCode: test.status, Body: objBody(codec, test.input)}, nil
				case p == test.calls[m] && m == "POST":
					return &http.Response{StatusCode: test.status, Body: objBody(codec, test.output)}, nil
				default:
					t.Fatalf("unexpected request: %#v\n%#v", req.URL, req)
					return nil, nil
				}
			}),
		}
		tf.Namespace = test.ns
		buf, exp := bytes.NewBuffer([]byte{}), bytes.NewBuffer([]byte{})

		cmd := NewCmdExposeService(f, buf)
		cmd.SetOutput(buf)
		for flag, value := range test.flags {
			cmd.Flags().Set(flag, value)
		}
		cmd.Run(cmd, test.args)

		f.PrintObject(cmd, test.output, exp)

		out, expectedOut := buf.String(), exp.String()
		if !reflect.DeepEqual(out, expectedOut) {
			t.Errorf("%s: Unexpected output! Expected\n%s\ngot\n%s", test.name, expectedOut, out)
		}
	}
}
