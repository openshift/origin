package resourceread

import (
	"encoding/json"

	configv1 "github.com/openshift/api/config/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var (
	configScheme = runtime.NewScheme()
	configCodecs = serializer.NewCodecFactory(configScheme)
)

func init() {
	utilruntime.Must(configv1.AddToScheme(configScheme))
}

func ReadFeatureGateV1(objBytes []byte) (*configv1.FeatureGate, error) {
	requiredObj, err := runtime.Decode(configCodecs.UniversalDecoder(configv1.SchemeGroupVersion), objBytes)
	if err != nil {
		return nil, err
	}

	return requiredObj.(*configv1.FeatureGate), nil
}

func ReadFeatureGateV1OrDie(objBytes []byte) *configv1.FeatureGate {
	requiredObj, err := ReadFeatureGateV1(objBytes)
	if err != nil {
		panic(err)
	}

	return requiredObj
}
func WriteFeatureGateV1(obj *configv1.FeatureGate) (string, error) {
	// done for pretty printing of JSON (technically also yaml)
	asMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return "", err
	}
	ret, err := json.MarshalIndent(asMap, "", "    ")
	if err != nil {
		return "", err
	}
	return string(ret) + "\n", nil
}

func WriteFeatureGateV1OrDie(obj *configv1.FeatureGate) string {
	ret, err := WriteFeatureGateV1(obj)
	if err != nil {
		panic(err)
	}
	return ret
}
