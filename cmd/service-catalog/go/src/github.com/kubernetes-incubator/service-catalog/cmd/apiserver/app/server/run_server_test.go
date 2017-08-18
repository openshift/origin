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
	"errors"
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/storage/tpr"

	"k8s.io/apimachinery/pkg/runtime"
	kubeclientfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
)

// make sure RunServer returns with an error when TPR fails to install
func TestRunServerInstallTPRFails(t *testing.T) {
	options := NewServiceCatalogServerOptions()

	fakeClientset := &kubeclientfake.Clientset{}
	fakeClientset.AddReactor("get", "thirdpartyresources", func(core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("TPR not found")
	})
	fakeClientset.AddReactor("create", "thirdpartyresources", func(core.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("Failed to create TPR")
	})

	options.StorageTypeString = "tpr"
	options.TPROptions = &TPROptions{
		"default-name-space",
		fakeClientset.Core().RESTClient(),
		installTPRsToCore(fakeClientset),
		"name-space",
	}

	err := RunServer(options)
	if _, ok := err.(tpr.ErrTPRInstall); !ok {
		t.Errorf("API Server did not report failure after failing to install Third Party Resources")
	}

	// make sure no more action after tpr failed to install
	getAction := 0
	createAction := 0
	actions := fakeClientset.Actions()
	for _, action := range actions {
		switch verb := action.GetVerb(); verb {
		case "get":
			getAction++
		case "create":
			createAction++
		default:
			t.Errorf("Unexpected action only 'get' and 'create' should be performed, got action: %s", verb)
		}

		if action.GetResource().Resource != "thirdpartyresources" {
			t.Errorf("Unexpected action performed after failing to install third party resource")
		}
	}

	if getAction != 4 {
		t.Errorf("Expected 4 'get' action, got %d", getAction)
	}

	if createAction != 4 {
		t.Errorf("Expected 4 'create' action, got %d", createAction)
	}
}
