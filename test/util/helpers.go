package util

import (
	"io/ioutil"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	kyaml "k8s.io/kubernetes/pkg/util/yaml"

	templateapi "github.com/openshift/origin/pkg/template/api"
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
