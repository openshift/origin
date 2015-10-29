package util

import (
	"fmt"
	"io/ioutil"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/latest"
	kyaml "k8s.io/kubernetes/pkg/util/yaml"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// CreateSampleImageStream creates an ImageStream in given namespace
func CreateSampleImageStream(namespace string) *imageapi.ImageStream {
	var stream imageapi.ImageStream
	jsonData, err := ioutil.ReadFile("fixtures/test-image-stream.json")
	if err != nil {
		fmt.Printf("ERROR: Unable to read: %v", err)
		return nil
	}
	latest.CodecForLegacyGroup().DecodeInto(jsonData, &stream)

	client, _ := GetClusterAdminClient(KubeConfigPath())
	result, err := client.ImageStreams(namespace).Create(&stream)
	if err != nil {
		fmt.Printf("ERROR: Unable to create sample ImageStream: %v\n", err)
		return nil
	}
	return result
}

// DeleteSampleImageStream removes the ImageStream created in given
// namespace
func DeleteSampleImageStream(stream *imageapi.ImageStream, namespace string) {
	client, _ := GetClusterAdminClient(KubeConfigPath())
	client.ImageStreams(namespace).Delete(stream.Name)
}

// GetBuildFixture reads the Build JSON and returns and Build object
func GetBuildFixture(filename string) *buildapi.Build {
	var build buildapi.Build
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("ERROR: Unable to read %s: %v", filename, err)
		return nil
	}
	latest.CodecForLegacyGroup().DecodeInto(jsonData, &build)
	return &build
}

func GetSecretFixture(filename string) *kapi.Secret {
	var secret kapi.Secret
	jsonData, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("ERROR: Unable to read %s: %v", filename, err)
		return nil
	}
	latest.CodecForLegacyGroup().DecodeInto(jsonData, &secret)
	return &secret
}

func GetTemplateFixture(filename string) (*templateapi.Template, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	jsonData, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	obj, err := latest.CodecForLegacyGroup().Decode(jsonData)
	if err != nil {
		return nil, err
	}
	return obj.(*templateapi.Template), nil
}
