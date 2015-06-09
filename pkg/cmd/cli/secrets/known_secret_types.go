package secrets

import (
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

type KnownSecretType struct {
	Type             kapi.SecretType
	RequiredContents util.StringSet
}

func (ks KnownSecretType) Matches(secretContent map[string][]byte) bool {
	if secretContent == nil {
		return false
	}

	secretKeys := util.KeySet(reflect.ValueOf(secretContent))
	return reflect.DeepEqual(ks.RequiredContents.List(), secretKeys.List())
}

var (
	KnownSecretTypes = []KnownSecretType{
		{kapi.SecretTypeDockercfg, util.NewStringSet(kapi.DockerConfigKey)},
	}
)
