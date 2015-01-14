package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&Secret{},
		&SecretList{},
	)
}

func (*Secret) IsAnAPIObject()     {}
func (*SecretList) IsAnAPIObject() {}
