package version

import (
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/kubernetes/pkg/version"
)

var OverrideGetVersionFn func() apimachineryversion.Info = nil

func getKubectlVersion() apimachineryversion.Info {
	if OverrideGetVersionFn != nil {
		return OverrideGetVersionFn()
	}

	return version.Get()
}
