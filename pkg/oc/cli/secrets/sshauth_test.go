package secrets

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

func TestValidateSSHAuth(t *testing.T) {
	tests := []struct {
		testName string
		args     []string
		options  func(genericclioptions.IOStreams) *CreateSSHAuthSecretOptions
		expErr   bool
	}{
		{
			testName: "validArgs",
			args:     []string{"testSecret"},
			options: func(streams genericclioptions.IOStreams) *CreateSSHAuthSecretOptions {
				o := NewCreateSSHAuthSecretOptions(streams)
				o.SecretName = "testSecret"
				o.PrivateKeyPath = "./bsFixtures/valid/ssh-privatekey"
				return o
			},
			expErr: false,
		},
		{
			testName: "validArgsWithCertificate",
			args:     []string{"testSecret"},
			options: func(streams genericclioptions.IOStreams) *CreateSSHAuthSecretOptions {
				o := NewCreateSSHAuthSecretOptions(streams)
				o.SecretName = "testSecret"
				o.PrivateKeyPath = "./bsFixtures/valid/ssh-privatekey"
				o.CertificatePath = "./bsFixtures/valid/ca.crt"
				return o
			},
			expErr: false,
		},
		{
			testName: "noName",
			args:     []string{},
			options: func(streams genericclioptions.IOStreams) *CreateSSHAuthSecretOptions {
				o := NewCreateSSHAuthSecretOptions(streams)
				o.SecretName = "testSecret"
				o.PrivateKeyPath = "./bsFixtures/valid/ssh-privatekey"
				o.CertificatePath = "./bsFixtures/valid/ca.crt"
				return o
			},
			expErr: true, //"Must have exactly one argument: secret name"
		},
		{
			testName: "noParams",
			args:     []string{"testSecret"},
			options: func(streams genericclioptions.IOStreams) *CreateSSHAuthSecretOptions {
				o := NewCreateSSHAuthSecretOptions(streams)
				o.SecretName = "testSecret"
				return o
			},
			expErr: true, //"Must provide SSH authentication credentials"
		},
	}

	for _, test := range tests {
		options := test.options(genericclioptions.NewTestIOStreamsDiscard())
		err := options.Validate(test.args)

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
		if secret.Type != corev1.SecretTypeSSHAuth {
			t.Errorf("%s: unexpected secret.Type: %v", test.testName, secret.Type)
		}
	}
}
