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

package binding

import (
	"fmt"
	"testing"

	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getTestInstanceCredential() *servicecatalog.ServiceInstanceCredential {
	return &servicecatalog.ServiceInstanceCredential{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Spec: servicecatalog.ServiceInstanceCredentialSpec{
			ServiceInstanceRef: v1.LocalObjectReference{
				Name: "some-string",
			},
		},
		Status: servicecatalog.ServiceInstanceCredentialStatus{
			Conditions: []servicecatalog.ServiceInstanceCredentialCondition{
				{
					Type:   servicecatalog.ServiceInstanceCredentialConditionReady,
					Status: servicecatalog.ConditionTrue,
				},
			},
		},
	}
}

func contextWithUserName(userName string) genericapirequest.Context {
	ctx := genericapirequest.NewContext()
	userInfo := &user.DefaultInfo{
		Name: userName,
	}
	return genericapirequest.WithUser(ctx, userInfo)
}

// TODO: Un-comment "spec-change" test case when there is a field
// in the spec to which the reconciler allows a change.

// TestInstanceCredentialUpdate tests that generation is incremented correctly when the
// spec of a ServiceInstanceCredential is updated.
func TestInstanceCredentialUpdate(t *testing.T) {
	cases := []struct {
		name                      string
		older                     *servicecatalog.ServiceInstanceCredential
		newer                     *servicecatalog.ServiceInstanceCredential
		shouldGenerationIncrement bool
	}{
		{
			name:  "no spec change",
			older: getTestInstanceCredential(),
			newer: getTestInstanceCredential(),
		},
		//		{
		//			name:  "spec change",
		//			older: getTestInstanceCredential(),
		//			newer: func() *v1alpha1.ServiceInstanceCredential {
		//				ic := getTestInstanceCredential()
		//				ic.Spec.ServiceInstanceRef = v1.LocalObjectReference{
		//					Name: "new-string",
		//				}
		//				return ic
		//			},
		//			shouldGenerationIncrement: true,
		//		},
	}
	for _, tc := range cases {
		bindingRESTStrategies.PrepareForUpdate(nil, tc.newer, tc.older)

		expectedGeneration := tc.older.Generation
		if tc.shouldGenerationIncrement {
			expectedGeneration = expectedGeneration + 1
		}
		if e, a := expectedGeneration, tc.newer.Generation; e != a {
			t.Errorf("%v: expected %v, got %v for generation", tc.name, e, a)
		}
	}
}

// TestInstanceCredentialUserInfo tests that the user info is set properly
// as the user changes for different modifications of the instance credential.
func TestInstanceCredentialUserInfo(t *testing.T) {
	// Enable the OriginatingIdentity feature
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))

	creatorUserName := "creator"
	createdInstanceCredential := getTestInstanceCredential()
	createContext := contextWithUserName(creatorUserName)
	bindingRESTStrategies.PrepareForCreate(createContext, createdInstanceCredential)

	if e, a := creatorUserName, createdInstanceCredential.Spec.UserInfo.Username; e != a {
		t.Errorf("unexpected user info in created spec: expected %q, got %q", e, a)
	}

	// TODO: Un-comment the following portion of this test when there is a field
	// in the spec to which the reconciler allows a change.

	//  updaterUserName := "updater"
	//	updatedInstanceCredential := getTestInstanceCredential()
	//	updateContext := contextWithUserName(updaterUserName)
	//	bindingRESTStrategies.PrepareForUpdate(updateContext, updatedInstanceCredential, createdInstanceCredential)

	//	if e, a := updaterUserName, updatedInstanceCredential.Spec.UserInfo.Username; e != a {
	//		t.Errorf("unexpected user info in updated spec: expected %q, got %q", e, a)
	//	}

	deleterUserName := "deleter"
	deletedInstanceCredential := getTestInstanceCredential()
	deleteContext := contextWithUserName(deleterUserName)
	bindingRESTStrategies.CheckGracefulDelete(deleteContext, deletedInstanceCredential, nil)

	if e, a := deleterUserName, deletedInstanceCredential.Spec.UserInfo.Username; e != a {
		t.Errorf("unexpected user info in deleted spec: expected %q, got %q", e, a)
	}
}
