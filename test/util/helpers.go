package util

import (
	"io/ioutil"

	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

func GetTemplateFixture(filename string) (*templateapi.Template, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	jsonData, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	obj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), jsonData)
	if err != nil {
		return nil, err
	}
	return obj.(*templateapi.Template), nil
}

func GetImageFixture(filename string) (*imageapi.Image, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	jsonData, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}
	obj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), jsonData)
	if err != nil {
		return nil, err
	}
	return obj.(*imageapi.Image), nil
}
