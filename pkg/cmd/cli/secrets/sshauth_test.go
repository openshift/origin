package secrets

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
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
				PrivateKeyPath: "./bsFixtures/valid/ssh-privatekey",
			},
			expErr: false,
		},
		{
			testName: "validArgsWithCertificate",
			args:     []string{"testSecret"},
			params: CreateSSHAuthSecretOptions{
				PrivateKeyPath:  "./bsFixtures/valid/ssh-privatekey",
				CertificatePath: "./bsFixtures/valid/ca.crt",
			},
			expErr: false,
		},
		{
			testName: "noName",
			args:     []string{},
			params: CreateSSHAuthSecretOptions{
				PrivateKeyPath:  "./bsFixtures/valid/ssh-privatekey",
				CertificatePath: "./bsFixtures/valid/ca.crt",
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
		err := options.Complete(nil, test.args)
		if err == nil {
			err = options.Validate()
		}

		if test.expErr {
			if err == nil {
				t.Errorf("%s: unexpected error: %v", test.testName, err)
			}
			continue
		}

		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}

		secret, err := options.NewSSHAuthSecret()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
		if secret.Type != api.SecretTypeSSHAuth {
			t.Errorf("%s: unexpected secret.Type: %v", test.testName, secret.Type)
		}
	}
}
