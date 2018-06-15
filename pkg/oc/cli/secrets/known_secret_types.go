package secrets

import (
	"reflect"

	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

type KnownSecretType struct {
	Type             kapi.SecretType
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
		{kapi.SecretTypeDockercfg, sets.NewString(kapi.DockerConfigKey)},
		{kapi.SecretTypeDockerConfigJson, sets.NewString(kapi.DockerConfigJsonKey)},
	}
)
