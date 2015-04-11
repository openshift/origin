package v1beta2

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypeWithName("v1beta2", "Config", &kapi.List{})
}
