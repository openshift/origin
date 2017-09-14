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

package controller

import (
	"encoding/json"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
)

const (
	originatingIdentityPlatform = "kubernetes"
	originatingIdentityUsername = "username"
	originatingIdentityUID      = "uid"
	originatingIdentityGroups   = "groups"
)

func buildOriginatingIdentity(userInfo *v1alpha1.UserInfo) (*osb.AlphaOriginatingIdentity, error) {
	if userInfo == nil {
		return nil, nil
	}
	oiFields := map[string]interface{}{}
	oiFields[originatingIdentityUsername] = userInfo.Username
	oiFields[originatingIdentityUID] = userInfo.UID
	oiFields[originatingIdentityGroups] = userInfo.Groups
	for k, v := range userInfo.Extra {
		oiFields[k] = v
	}
	oiValue, err := json.Marshal(oiFields)
	if err != nil {
		return nil, err
	}
	oi := &osb.AlphaOriginatingIdentity{
		Platform: originatingIdentityPlatform,
		Value:    string(oiValue),
	}
	return oi, nil
}
