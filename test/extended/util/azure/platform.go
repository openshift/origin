package azure

import (
	"strings"

	v1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/objx"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

// SkipUnlessPlatformAzure if cluster is deployed in
// Azure cloud else skip the tests.
func SkipUnlessPlatformAzure(infra objx.Map) {
	platform := infra.Get("status.platformStatus.type")
	if !strings.EqualFold(platform.Str(), string(v1.AzurePlatformType)) {
		e2eskipper.Skipf("not desired platform type")
	}
}

// GetInfrastructureName returns the InfrastructureName as defined
// in the infrastructure/cluster resource. Will return empty
// on encountering any errors or some issue in the resource itself.
func GetInfrastructureName(infra objx.Map) string {
	platform := infra.Get("status.InfrastructureName")
	return platform.Str()
}

// GetInfraResourceTags returns the list of tags present in
// infrastructure/cluster resource.
func GetInfraResourceTags(infra objx.Map) (tags map[string]string) {
	platform := infra.Get("status.platformStatus.azure.ResourceTags")
	platformTags := platform.InterSlice()
	if platformTags != nil {
		tags = make(map[string]string, len(platformTags))
		for _, item := range platformTags {
			tag := item.(v1.AzureResourceTag)
			tags[tag.Key] = tag.Value
		}
	}
	return
}
