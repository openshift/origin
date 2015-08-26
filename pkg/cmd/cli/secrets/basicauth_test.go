package secrets

import (
	"testing"
)

func TestValidateBasicAuth(t *testing.T) {
	tests := []struct {
		testName string
		args     []string
		params   CreateBasicAuthSecretOptions
		expErr   bool
	}{
		{
			testName: "validArgs",
			args:     []string{"testSecret"},
			params: CreateBasicAuthSecretOptions{
				Username: "testUser",
				Password: "testPassword",
			},
			expErr: false,
		},
		{
			testName: "validArgsWithCertificate",
			args:     []string{"testSecret"},
			params: CreateBasicAuthSecretOptions{
				Username:        "testUser",
				Password:        "testPassword",
				CertificatePath: "./bsFixtures/valid/ca.crt",
			},
			expErr: false,
		},
		{
			testName: "validArgsWithGitconfig",
			args:     []string{"testSecret"},
			params: CreateBasicAuthSecretOptions{
				Username:      "testUser",
				Password:      "testPassword",
				GitConfigPath: "./bsFixtures/leadingdot/.gitconfig",
			},
			expErr: false,
		},
		{
			testName: "noName",
			args:     []string{},
			params: CreateBasicAuthSecretOptions{
				Username: "testUser",
				Password: "testPassword",
			},
			expErr: true, //"Must have exactly one argument: secret name"
		},
		{
			testName: "noParams",
			args:     []string{"testSecret"},
			params:   CreateBasicAuthSecretOptions{},
			expErr:   true, //"Must provide basic authentication credentials"
		},
		{
			testName: "passwordAndPrompt",
			args:     []string{"testSecret"},
			params: CreateBasicAuthSecretOptions{
				Username:          "testUser",
				Password:          "testPassword",
				PromptForPassword: true,
			},
			expErr: true, //"Must provide either --prompt or --password flag"
		},
	}

	for _, test := range tests {
		options := test.params
		options.Complete(nil, test.args)
		err := options.Validate()
		if err != nil && !test.expErr {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
	}
}
