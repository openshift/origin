package v1beta2

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypeWithName("v1beta2", "Config", &List{})
}

func (*List) IsAnAPIObject() {}
