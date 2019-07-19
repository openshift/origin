package util

import (
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/apitesting"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	templatev1 "github.com/openshift/api/template/v1"
)

const additionalAllowedRegistriesEnvVar = "ADDITIONAL_ALLOWED_REGISTRIES"

func GetTemplateFixture(filename string) (*templatev1.Template, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	jsonData, err := kyaml.ToJSON(data)
	if err != nil {
		return nil, err
	}

	_, codecFactory := apitesting.SchemeForOrDie(templatev1.Install)
	obj, err := runtime.Decode(codecFactory.UniversalDeserializer(), jsonData)
	if err != nil {
		return nil, err
	}
	return obj.(*templatev1.Template), nil
}

func SetAdditionalAllowedRegistries(hostPortGlobs ...string) {
	os.Setenv(additionalAllowedRegistriesEnvVar, strings.Join(hostPortGlobs, ","))
}

func AddAdditionalAllowedRegistries(hostPortGlobs ...string) {
	regs := GetAdditionalAllowedRegistries()
	regs.Insert(hostPortGlobs...)
	SetAdditionalAllowedRegistries(regs.List()...)
}

func GetAdditionalAllowedRegistries() sets.String {
	regs := sets.NewString()
	for _, r := range regexp.MustCompile(`[[:space:],]+`).Split(os.Getenv(additionalAllowedRegistriesEnvVar), -1) {
		if len(r) > 0 {
			regs.Insert(r)
		}
	}
	return regs
}
