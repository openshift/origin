package app

import (
	"github.com/golang/glog"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
)

var (
	// OpenshiftVersionInfo exposes openshift verison info to k8s packages
	OpenshiftVersionInfo *apimachineryversion.Info
)

// GetOpenshiftVersion returns Overall Openshift codebase version
func GetOpenshiftVersion(componentKey string) *apimachineryversion.Info {
	if OpenshiftVersionInfo == nil {
		glog.Fatalf("Can not find Openshift version for retrieving matching images. Openshift can be additionally started with %s environment variable to find matching image", componentKey)
	}
	return OpenshiftVersionInfo
}
