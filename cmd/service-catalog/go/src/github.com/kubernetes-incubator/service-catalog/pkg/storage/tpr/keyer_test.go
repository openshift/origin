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

package tpr

import (
	"testing"

	"k8s.io/apiserver/pkg/endpoints/request"
)

const (
	defaultCtxNS = "defaultTestNS"
	ctxNS        = "testNS"
	separator    = "/"
	resourceName = "myResource"
)

func TestKeyRoot(t *testing.T) {
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, ctxNS)
	keyer := Keyer{DefaultNamespace: defaultCtxNS}
	root := keyer.KeyRoot(ctx)
	if root != ctxNS {
		t.Fatalf("key root '%s' wasn't expected '%s'", root, ctxNS)
	}
	ctx = request.NewContext()
	root = keyer.KeyRoot(ctx)
	if root != keyer.DefaultNamespace {
		t.Fatalf("key root '%s' wasn't expected '%s'", root, keyer.DefaultNamespace)
	}
}

func TestKey(t *testing.T) {
	ctx := request.NewContext()
	ctx = request.WithNamespace(ctx, ctxNS)
	keyer := Keyer{Separator: separator, ResourceName: resourceName}
	key, err := keyer.Key(ctx, resourceName)
	if err != nil {
		t.Fatalf("key func returned an error %s", err)
	}
	expected := ctxNS + separator + resourceName
	if key != expected {
		t.Fatalf("key was '%s', not expected '%s', key, expected", key, expected)
	}
}

func TestNamespaceAndNameFromKey(t *testing.T) {
	const testName = "testName"
	keyer := Keyer{Separator: separator, ResourceName: resourceName}
	key := ctxNS + separator + testName
	ns, name, err := keyer.NamespaceAndNameFromKey(key)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if ns != ctxNS {
		t.Fatalf("namespace was '%s', not expected '%s'", ns, ctxNS)
	}
	if name != testName {
		t.Fatalf("name was '%s', not expected '%s'", name, testName)
	}

	key = ctxNS
	ns, name, err = keyer.NamespaceAndNameFromKey(key)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if ns != ctxNS {
		t.Fatalf("namespace was '%s', not expected '%s'", ns, ctxNS)
	}
	if name != "" {
		t.Fatalf("expected empty name, got '%s'", name)
	}

	key = "a" + separator + "b" + separator + "c"
	ns, name, err = keyer.NamespaceAndNameFromKey(key)
	if err == nil {
		t.Fatalf("expected error")
	}
	if ns != "" {
		t.Fatalf("expected empty namespace")
	}
	if name != "" {
		t.Fatalf("expected empty name")
	}

}
