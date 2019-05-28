package secrets

import (
	"reflect"

	coreapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type KnownSecretType struct {
	Type             coreapiv1.SecretType
	RequiredContents sets.String
}

func (ks KnownSecretType) Matches(secretContent map[string][]byte) bool {
	if secretContent == nil {
		return false
	}
	secretKeys := sets.StringKeySet(secretContent)
	return reflect.DeepEqual(ks.RequiredContents.List(), secretKeys.List())
}

var (
	KnownSecretTypes = []KnownSecretType{
		{coreapiv1.SecretTypeDockercfg, sets.NewString(coreapiv1.DockerConfigKey)},
		{coreapiv1.SecretTypeDockerConfigJson, sets.NewString(coreapiv1.DockerConfigJsonKey)},
	}
)
