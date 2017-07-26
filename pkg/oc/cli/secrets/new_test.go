package secrets

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
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
	os.Symlink(".", "./bsFixtures/dir/symbolic")
	defer os.Remove("./bsFixtures/dir/symbolic")

	tests := []struct {
		testName string
		args     []string
		expErr   bool
		quiet    bool

		errStreamContent string
	}{
		{
			testName: "validSources",
			args:     []string{"testSecret", "./bsFixtures/www.google.com", "./bsFixtures/dirNoSubdir"},
		},
		{
			testName: "allowsMixedCaseAndDash",
			args:     []string{"testSecret", "./bsFixtures/invalid/invalid-DNS"},
		},
		{
			testName: "failsWithUnderscore",
			args:     []string{"testSecret", "./bsFixtures/invalid/not\\valid"},
			expErr:   true,
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
			testName:         "testQuietFalse",
			args:             []string{"testSecret", "./bsFixtures/dir"},
			errStreamContent: "Skipping resource bsFixtures/dir/symbolic\n",
		},
		{
			testName: "testNamedKeys",
			args:     []string{"testSecret", ".googlename=./bsFixtures/www.google.com"},
			expErr:   false,
		},
		{
			testName: "testNamedDir",
			args:     []string{"testSecret", ".somename=./bsFixtures/dirNoSubdir"},
			expErr:   true, // "Cannot give a key name for a directory path."
		},
		{
			testName: "testUnnamedDir",
			args:     []string{"testSecret", "./bsFixtures/dirContainsMany"},
			expErr:   false,
		},
		{
			testName: "testMalformedName",
			args:     []string{"testSecret", ".google=name=./bsFixtures/www.google.com"},
			expErr:   true, // "Key names or file paths cannot contain '='."
		},
		{
			testName: "testMissingName",
			args:     []string{"testSecret", "=./bsFixtures/www.google.com"},
			expErr:   true, // "Key name for file path ./bsFixtures/www.google.com missing."
		},
		{
			testName: "testMissingPath",
			args:     []string{"testSecret", ".somename="},
			expErr:   true, // "File path for key name some-name missing."
		},
		{
			testName: "testNamesAvoidCollision",
			args:     []string{"testSecret", ".googlename=./bsFixtures/www.google.com", ".othergooglename=./bsFixtures/multiple/www.google.com"},
			expErr:   false,
		},
		{
			testName: "testNameCollision",
			args:     []string{"testSecret", ".googlename=./bsFixtures/www.google.com", ".googlename=./bsFixtures/multiple/www.google.com"},
			expErr:   true, // "Cannot add key google-name from path ./bsFixtures/multiple/www.google.com, another key by that name already exists."
		},
	}
	for _, test := range tests {
		errStream := &bytes.Buffer{}
		options := NewCreateSecretOptions()
		options.Stderr = errStream
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
		if err == nil && test.expErr {
			t.Errorf("%s: missing expected error", test.testName)
		}
		if string(errStream.Bytes()) != test.errStreamContent {
			t.Errorf("%s: expected %s, got %v", test.testName, test.errStreamContent, string(errStream.Bytes()))
		}
	}
}

func TestSecretTypeSpecified(t *testing.T) {
	options := CreateSecretOptions{
		Name:           "any",
		SecretTypeName: string(kapi.SecretTypeDockercfg),
		Sources:        []string{"./bsFixtures/www.google.com"},
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
		Sources: []string{"./bsFixtures/leadingdot/.dockercfg"},
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
