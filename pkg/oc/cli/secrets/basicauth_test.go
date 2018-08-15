package secrets

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

func TestValidateBasicAuth(t *testing.T) {
	tests := []struct {
		testName string
		options  func(genericclioptions.IOStreams) *CreateBasicAuthSecretOptions
		expErr   bool
	}{
		{
			testName: "validArgs",
			options: func(streams genericclioptions.IOStreams) *CreateBasicAuthSecretOptions {
				o := NewCreateBasicAuthSecretOptions(streams)
				o.Username = "testUser"
				o.Password = "testPassword"
				o.SecretName = "testSecret"
				return o
			},
			expErr: false,
		},
		{
			testName: "validArgsWithCertificate",
			options: func(streams genericclioptions.IOStreams) *CreateBasicAuthSecretOptions {
				o := NewCreateBasicAuthSecretOptions(streams)
				o.Username = "testUser"
				o.Password = "testPassword"
				o.SecretName = "testSecret"
				o.CertificatePath = "./bsFixtures/valid/ca.crt"
				return o
			},
			expErr: false,
		},
		{
			testName: "validArgsWithGitconfig",
			options: func(streams genericclioptions.IOStreams) *CreateBasicAuthSecretOptions {
				o := NewCreateBasicAuthSecretOptions(streams)
				o.Username = "testUser"
				o.Password = "testPassword"
				o.SecretName = "testSecret"
				o.GitConfigPath = "./bsFixtures/leadingdot/.gitconfig"
				return o
			},
			expErr: false,
		},
		{
			testName: "noName",
			options: func(streams genericclioptions.IOStreams) *CreateBasicAuthSecretOptions {
				o := NewCreateBasicAuthSecretOptions(streams)
				o.Username = "testUser"
				o.Password = "testPassword"
				return o
			},
			expErr: true, //"Must have exactly one argument: secret name"
		},
		{
			testName: "noParams",
			options: func(streams genericclioptions.IOStreams) *CreateBasicAuthSecretOptions {
				o := NewCreateBasicAuthSecretOptions(streams)
				o.SecretName = "testSecret"
				return o
			},
			expErr: true, //"Must provide basic authentication credentials"
		},
	}

	for _, test := range tests {
		options := test.options(genericclioptions.NewTestIOStreamsDiscard())
		err := options.Validate()
		if test.expErr {
			if err == nil {
				t.Errorf("%s: unexpected error: %v", test.testName, err)
			}
			continue
		}

		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
			continue
		}

		secret, err := options.NewBasicAuthSecret()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
		if secret.Type != corev1.SecretTypeBasicAuth {
			t.Errorf("%s: unexpected secret.Type: %v", test.testName, secret.Type)
		}
	}
}
