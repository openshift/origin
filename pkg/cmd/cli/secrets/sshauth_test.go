package secrets

import (
	"testing"
)

func TestValidateSSHAuth(t *testing.T) {
	tests := []struct {
		testName string
		args     []string
		params   CreateSSHAuthSecretOptions
		expErr   bool
	}{
		{
			testName: "validArgs",
			args:     []string{"testSecret"},
			params: CreateSSHAuthSecretOptions{
				PrivateKeyPath: "../../../../test/fixtures/placeholders/valid/ssh-privatekey",
			},
			expErr: false,
		},
		{
			testName: "validArgsWithCertificate",
			args:     []string{"testSecret"},
			params: CreateSSHAuthSecretOptions{
				PrivateKeyPath:  "../../../../test/fixtures/placeholders/valid/ssh-privatekey",
				CertificatePath: "../../../../test/fixtures/placeholders/valid/ca.crt",
			},
			expErr: false,
		},
		{
			testName: "noName",
			args:     []string{},
			params: CreateSSHAuthSecretOptions{
				PrivateKeyPath:  "../../../../test/fixtures/placeholders/valid/ssh-privatekey",
				CertificatePath: "../../../../test/fixtures/placeholders/valid/ca.crt",
			},
			expErr: true, //"Must have exactly one argument: secret name"
		},
		{
			testName: "noParams",
			args:     []string{"testSecret"},
			params:   CreateSSHAuthSecretOptions{},
			expErr:   true, //"Must provide SSH authentication credentials"
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
