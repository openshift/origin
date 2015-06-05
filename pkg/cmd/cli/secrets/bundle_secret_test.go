package secrets

import (
	"io/ioutil"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		testName string
		args     []string
		expErr   bool
	}{
		{
			testName: "validArgs",
			args:     []string{"testSecret", "./bsFixtures/www.google.com"},
		},
		{
			testName: "noName",
			args:     []string{"./bsFixtures/www.google.com"},
			expErr:   true, //"Secret name is required"
		},
		{
			testName: "noFilesPassed",
			args:     []string{"testSecret"},
			expErr:   true, //"At least one source file or directory must be specified"
		},
	}

	for _, test := range tests {
		options := NewCreateSecretOptions()
		options.Complete(test.args, nil)
		err := options.Validate()
		if err != nil && !test.expErr {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
	}
}

func TestCreateSecret(t *testing.T) {
	tests := []struct {
		testName string
		args     []string
		expErr   bool
		quiet    bool
	}{
		{
			testName: "validSources",
			args:     []string{"testSecret", "./bsFixtures/www.google.com", "./bsFixtures/dirNoSubdir"},
		},
		{
			testName: "invalidDNS",
			args:     []string{"testSecret", "./bsFixtures/invalid/invalid-DNS"},
			expErr:   true, // "/bsFixtures/invalid-DNS cannot be used as a key in a secret"
		},
		{
			testName: "leadingDotsAllowed",
			args:     []string{"testSecret", "./bsFixtures/leadingdot/.dockercfg"},
		},
		{
			testName: "filesSameName",
			args:     []string{"testSecret", "./bsFixtures/www.google.com", "./bsFixtures/multiple/www.google.com"},
			expErr:   true, // "Multiple files with the same name (www.google.com) cannot be included a secret"
		},
		{
			testName: "testQuietTrue",
			args:     []string{"testSecret", "./bsFixtures/dir"},
			quiet:    true,
		},
		{
			testName: "testQuietFalse",
			args:     []string{"testSecret", "./bsFixtures/dir"},
			expErr:   true, // "Skipping resource <resource path>"
		},
	}
	for _, test := range tests {
		options := NewCreateSecretOptions()
		options.Complete(test.args, nil)
		options.Quiet = test.quiet

		err := options.Validate()
		if err != nil {
			t.Errorf("unexpected error")
		}
		_, err = options.BundleSecret()
		if err != nil && !test.expErr {
			t.Errorf("%s: unexpected error: %s", test.testName, err)
		}
	}
}

func TestSecretTypeSpecified(t *testing.T) {
	options := CreateSecretOptions{
		Name:           "any",
		SecretTypeName: string(kapi.SecretTypeDockercfg),
		Sources:        util.StringList([]string{"./bsFixtures/www.google.com"}),
		Stderr:         ioutil.Discard,
	}

	secret, err := options.BundleSecret()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if secret.Type != kapi.SecretTypeDockercfg {
		t.Errorf("expected %v, got %v", kapi.SecretTypeDockercfg, secret.Type)
	}
}
func TestSecretTypeDiscovered(t *testing.T) {
	options := CreateSecretOptions{
		Name:    "any",
		Sources: util.StringList([]string{"./bsFixtures/leadingdot/.dockercfg"}),
		Stderr:  ioutil.Discard,
	}

	secret, err := options.BundleSecret()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if secret.Type != kapi.SecretTypeDockercfg {
		t.Errorf("expected %v, got %v", kapi.SecretTypeDockercfg, secret.Type)
	}
}
