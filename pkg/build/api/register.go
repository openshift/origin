package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

func init() {
	runtime.AddKnownTypes("",
		Build{},
		BuildList{},
		BuildConfig{},
		BuildConfigList{},
	)
}
