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
	"errors"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	kubeclientfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	core "k8s.io/client-go/testing"
)

func setup(getFn, createFn func(core.Action) (bool, runtime.Object, error)) *kubeclientfake.Clientset {
	fakeClientset := &kubeclientfake.Clientset{}

	fakeClientset.AddReactor("get", "thirdpartyresources", getFn)

	fakeClientset.AddReactor("create", "thirdpartyresources", createFn)

	return fakeClientset
}

//make sure all resources types are installed
func TestInstallTypesAllResources(t *testing.T) {
	getCallCount := 0
	createCallCount := 0

	fakeClientset := setup(
		func(core.Action) (bool, runtime.Object, error) {
			getCallCount++
			// if 'create' has been called on all tprs, return 'nil' error to indicate tpr is created
			if createCallCount == len(thirdPartyResources) {
				return true, &v1beta1.ThirdPartyResource{}, nil
			}

			// return error to indicate tpr is not found
			return true, nil, errors.New("Resource not found")
		},
		func(core.Action) (bool, runtime.Object, error) {
			createCallCount++
			return true, nil, nil
		},
	)

	if err := InstallTypes(fakeClientset.Extensions().ThirdPartyResources()); err != nil {
		t.Fatalf("error installing types (%s)", err)
	}

	expectTotal := len(thirdPartyResources)
	if createCallCount != expectTotal {
		t.Errorf("Expected %d Third Party Resources created instead of %d", expectTotal, createCallCount)
	}
}

//make sure to skip resource that is already installed
func TestInstallTypesResourceExisted(t *testing.T) {
	getCallCount := 0
	createCallCount := 0
	createCallArgs := []string{}

	fakeClientset := setup(
		func(core.Action) (bool, runtime.Object, error) {
			getCallCount++
			if getCallCount == 1 {
				// return broker TPR on 1st call to indicate broker TPR exists
				return true, &serviceBrokerTPR, nil
			} else if createCallCount == len(thirdPartyResources)-1 {
				// once 'create' has been called on all tprs, return 'nil' error to indicate tpr is created
				return true, &v1beta1.ThirdPartyResource{}, nil
			}

			return true, nil, errors.New("Resource not found")
		},
		func(action core.Action) (bool, runtime.Object, error) {
			createCallCount++
			createCallArgs = append(createCallArgs, action.(core.CreateAction).GetObject().(*v1beta1.ThirdPartyResource).Name)
			return true, nil, nil
		},
	)

	if err := InstallTypes(fakeClientset.Extensions().ThirdPartyResources()); err != nil {
		t.Fatalf("error installing (%s)", err)
	}

	if createCallCount != len(thirdPartyResources)-1 {
		t.Errorf("Failed to skip 1 installed Third Party Resource")
	}

	for _, name := range createCallArgs {
		if name == serviceBrokerTPR.Name {
			t.Errorf("Failed to skip installing 'broker' as Third Party Resource as it already existed")
		}
	}
}

//make sure all errors are received for all failed install
func TestInstallTypesErrors(t *testing.T) {
	getCallCount := 0
	createCallCount := 0

	fakeClientset := setup(
		func(core.Action) (bool, runtime.Object, error) {
			getCallCount++
			// if 'create' has been called on all tprs, return 'nil' error to indicate tpr is created
			if createCallCount == len(thirdPartyResources) {
				return true, &v1beta1.ThirdPartyResource{}, nil
			}

			// return error to indicate tpr is not found
			return true, nil, errors.New("Resource not found")
		},
		func(core.Action) (bool, runtime.Object, error) {
			createCallCount++
			if createCallCount <= 2 {
				return true, nil, errors.New("Error " + strconv.Itoa(createCallCount))
			}
			return true, nil, nil
		},
	)

	err := InstallTypes(fakeClientset.Extensions().ThirdPartyResources())

	errStr := err.Error()
	if !strings.Contains(errStr, "Error 1") && !strings.Contains(errStr, "Error 2") {
		t.Errorf("Failed to receive correct errors during installation of Third Party Resource concurrently, error received: %s", errStr)
	}
}

//make sure we don't poll on resource that was failed on install
func TestInstallTypesPolling(t *testing.T) {
	getCallCount := 0
	createCallCount := 0
	getCallArgs := []string{}

	fakeClientset := setup(
		func(action core.Action) (bool, runtime.Object, error) {
			getCallCount++
			if getCallCount > len(thirdPartyResources) {
				getCallArgs = append(getCallArgs, action.(core.GetAction).GetName())
				return true, &v1beta1.ThirdPartyResource{}, nil
			}

			return true, nil, errors.New("Resource not found")
		},
		func(action core.Action) (bool, runtime.Object, error) {
			createCallCount++
			name := action.(core.CreateAction).GetObject().(*v1beta1.ThirdPartyResource).Name
			if name == serviceBrokerTPR.Name || name == serviceInstanceTPR.Name {
				return true, nil, errors.New("Error creating TPR")
			}
			return true, nil, nil
		},
	)

	if err := InstallTypes(fakeClientset.Extensions().ThirdPartyResources()); err == nil {
		t.Fatal("InstallTypes was supposed to error but didn't")
	}

	for _, name := range getCallArgs {
		if name == serviceBrokerTPR.Name || name == serviceInstanceTPR.Name {
			t.Errorf("Failed to skip polling for resource that failed to install")
		}
	}
}
