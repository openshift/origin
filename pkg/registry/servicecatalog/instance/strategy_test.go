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

package instance

import (
	"fmt"
	"testing"

	utilfeature "k8s.io/apiserver/pkg/util/feature"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	scfeatures "github.com/kubernetes-incubator/service-catalog/pkg/features"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
)

func getTestInstance() *servicecatalog.ServiceInstance {
	return &servicecatalog.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
		},
		Spec: servicecatalog.ServiceInstanceSpec{
			ServiceClassName: "test-serviceclass",
			PlanName:         "test-plan",
			UserInfo: &servicecatalog.UserInfo{
				Username: "some-user",
			},
		},
		Status: servicecatalog.ServiceInstanceStatus{
			Conditions: []servicecatalog.ServiceInstanceCondition{
				{
					Type:   servicecatalog.ServiceInstanceConditionReady,
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

// TestInstanceUpdate tests that generation is incremented correctly when the
// spec of a Instance is updated.
func TestInstanceUpdate(t *testing.T) {
	cases := []struct {
		name                      string
		older                     *servicecatalog.ServiceInstance
		newer                     *servicecatalog.ServiceInstance
		shouldGenerationIncrement bool
	}{
		{
			name:  "no spec change",
			older: getTestInstance(),
			newer: getTestInstance(),
		},
		//		{
		//			name:  "spec change",
		//			older: getTestInstance(),
		//			newer: func() *servicecatalog.ServiceInstance {
		//				i := getTestInstance()
		//				i.Spec.ServiceClassName = "new-serviceclass"
		//				return i
		//			},
		//			shouldGenerationIncrement: true,
		//		},
	}

	for _, tc := range cases {
		instanceRESTStrategies.PrepareForUpdate(nil, tc.newer, tc.older)

		expectedGeneration := tc.older.Generation
		if tc.shouldGenerationIncrement {
			expectedGeneration = expectedGeneration + 1
		}
		if e, a := expectedGeneration, tc.newer.Generation; e != a {
			t.Errorf("%v: expected %v, got %v for generation", tc.name, e, a)
		}
	}
}

// TestInstanceUserInfo tests that the user info is set properly
// as the user changes for different modifications of the instance.
func TestInstanceUserInfo(t *testing.T) {
	// Enable the OriginatingIdentity feature
	utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=true", scfeatures.OriginatingIdentity))
	defer utilfeature.DefaultFeatureGate.Set(fmt.Sprintf("%v=false", scfeatures.OriginatingIdentity))

	creatorUserName := "creator"
	createdInstance := getTestInstance()
	createContext := contextWithUserName(creatorUserName)
	instanceRESTStrategies.PrepareForCreate(createContext, createdInstance)

	if e, a := creatorUserName, createdInstance.Spec.UserInfo.Username; e != a {
		t.Errorf("unexpected user info in created spec: expected %v, got %v", e, a)
	}

	// TODO: Un-comment the following portion of this test when there is a field
	// in the spec to which the reconciler allows a change.

	//  updaterUserName := "updater"
	//	updatedInstance := getTestInstance()
	//	updateContext := contextWithUserName(updaterUserName)
	//	instanceRESTStrategies.PrepareForUpdate(updateContext, updatedInstance, createdInstance)

	//	if e, a := updaterUserName, updatedInstance.Spec.UserInfo.Username; e != a {
	//		t.Errorf("unexpected user info in updated spec: expected %v, got %v", e, a)
	//	}

	deleterUserName := "deleter"
	deletedInstance := getTestInstance()
	deleteContext := contextWithUserName(deleterUserName)
	instanceRESTStrategies.CheckGracefulDelete(deleteContext, deletedInstance, nil)

	if e, a := deleterUserName, deletedInstance.Spec.UserInfo.Username; e != a {
		t.Errorf("unexpected user info in deleted spec: expected %v, got %v", e, a)
	}
}
