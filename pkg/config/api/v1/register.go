package v1beta3

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypeWithName("v1beta3", "Config", &List{})
}

func (*List) IsAnAPIObject() {}
