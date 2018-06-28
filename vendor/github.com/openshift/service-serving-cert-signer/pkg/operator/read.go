package operator

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	scsv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
)

var (
	configScheme = runtime.NewScheme()
	configCodecs = serializer.NewCodecFactory(configScheme)
)

func init() {
	scsv1alpha1.AddToScheme(configScheme)
}

func readServiceServingCertSignerConfig(objBytes []byte) (*unstructured.Unstructured, error) {
	data, err := kyaml.ToJSON(objBytes)
	if err != nil {
		panic(err)
	}
	defaultConfigObj, err := runtime.Decode(unstructured.UnstructuredJSONScheme, data)
	if err != nil {
		return nil, err
	}
	ret, ok := defaultConfigObj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("expected *unstructured.Unstructured, got %T", defaultConfigObj)
	}

	return ret, nil
}