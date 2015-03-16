package util

import (
	"fmt"
	"io/ioutil"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// CreateSampleImageRepository creates a ImageRepository in given namespace
func CreateSampleImageRepository(namespace string) *imageapi.ImageRepository {
	var repo imageapi.ImageRepository
	jsonData, err := ioutil.ReadFile("fixtures/sample-image-repository.json")
	if err != nil {
		fmt.Printf("ERROR: Unable to read: %v", err)
		return nil
	}
	latest.Codec.DecodeInto(jsonData, &repo)

	client, _ := GetClusterAdminClient(KubeConfigPath())
	result, err := client.ImageRepositories(namespace).Create(&repo)
	if err != nil {
		fmt.Printf("ERROR: Unable to create sample ImageRepository: %v\n", err)
		return nil
	}
	return result
}

// DeleteSampleImageRepository removes the ImageRepository created in given
// namespace
func DeleteSampleImageRepository(repo *imageapi.ImageRepository, namespace string) {
	client, _ := GetClusterAdminClient(KubeConfigPath())
	client.ImageRepositories(namespace).Delete(repo.Name)
}

// GetBuildFixture reads the Build JSON and returns and Build object
func GetBuildFixture(filename string) *buildapi.Build {
	var build buildapi.Build
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("ERROR: Unable to read %s: %v", filename, err)
		return nil
	}
	latest.Codec.DecodeInto(jsonData, &build)
	return &build
}
