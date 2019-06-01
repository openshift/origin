package oauthserver

import (
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	osinv1 "github.com/openshift/api/osin/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/yaml"
)

func TestGetDefaultSessionSecrets(t *testing.T) {
	secrets, err := getSessionSecrets("")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(secrets) != 2 {
		t.Errorf("Unexpected 2 secrets, got: %#v", secrets)
	}
}

func TestGetMissingSessionSecretsFile(t *testing.T) {
	_, err := getSessionSecrets("missing")
	if err == nil {
		t.Errorf("Expected error, got none")
	}
}

func TestGetInvalidSessionSecretsFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "invalid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	ioutil.WriteFile(tmpfile.Name(), []byte("invalid content"), os.FileMode(0600))

	_, err = getSessionSecrets(tmpfile.Name())
	if err == nil {
		t.Errorf("Expected error, got none")
	}
}

func TestGetEmptySessionSecretsFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "empty.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	secrets := &osinv1.SessionSecrets{
		Secrets: []osinv1.SessionSecret{},
	}

	yaml, err := WriteYAML(secrets, osinv1.Install)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	ioutil.WriteFile(tmpfile.Name(), []byte(yaml), os.FileMode(0600))

	_, err = getSessionSecrets(tmpfile.Name())
	if err == nil {
		t.Errorf("Expected error, got none")
	}
}

func TestGetValidSessionSecretsFile(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	secrets := &osinv1.SessionSecrets{
		Secrets: []osinv1.SessionSecret{
			{Authentication: "a1", Encryption: "e1"},
			{Authentication: "a2", Encryption: "e2"},
		},
	}
	expectedSecrets := [][]byte{[]byte("a1"), []byte("e1"), []byte("a2"), []byte("e2")}

	yaml, err := WriteYAML(secrets, osinv1.Install)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	ioutil.WriteFile(tmpfile.Name(), []byte(yaml), os.FileMode(0600))

	readSecrets, err := getSessionSecrets(tmpfile.Name())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !reflect.DeepEqual(readSecrets, expectedSecrets) {
		t.Errorf("Unexpected %v, got %v", expectedSecrets, readSecrets)
	}
}

type InstallFunc func(scheme *runtime.Scheme) error

func WriteYAML(obj runtime.Object, schemeFns ...InstallFunc) ([]byte, error) {
	scheme := runtime.NewScheme()
	for _, schemeFn := range schemeFns {
		err := schemeFn(scheme)
		if err != nil {
			return nil, err
		}
	}
	codec := serializer.NewCodecFactory(scheme).LegacyCodec(scheme.PrioritizedVersionsAllGroups()...)

	json, err := runtime.Encode(codec, obj)
	if err != nil {
		return nil, err
	}

	content, err := yaml.JSONToYAML(json)
	if err != nil {
		return nil, err
	}
	return content, err
}
