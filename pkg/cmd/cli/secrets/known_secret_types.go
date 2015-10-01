package secrets

import (
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/sets"
)

type KnownSecretType struct {
	Type             kapi.SecretType
	RequiredContents sets.String
}

func (ks KnownSecretType) Matches(secretContent map[string][]byte) bool {
	if secretContent == nil {
		return false
	}
	secretKeys := sets.KeySet(reflect.ValueOf(secretContent))
	return reflect.DeepEqual(ks.RequiredContents.List(), secretKeys.List())
}

var (
	KnownSecretTypes = []KnownSecretType{
		{kapi.SecretTypeDockercfg, sets.NewString(kapi.DockerConfigKey)},
	}
)
