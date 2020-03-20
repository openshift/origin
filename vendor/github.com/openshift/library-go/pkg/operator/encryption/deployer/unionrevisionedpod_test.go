package deployer_test

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/diff"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/encryption/deployer"
	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	"github.com/openshift/library-go/pkg/operator/encryption/statemachine"
	encryptiontesting "github.com/openshift/library-go/pkg/operator/encryption/testing"
)

func TestUnionRevisionLabelPodDeployer(t *testing.T) {
	scenarios := []struct {
		name      string
		deployers []statemachine.Deployer

		expectedSecret    *corev1.Secret
		expectedConverged bool
		expectedErr       bool
	}{
		{
			name: "happy path",
			deployers: []statemachine.Deployer{
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), true, nil),
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), true, nil),
			},
			expectedSecret:    createDefaultSecretWithEncryptionConfig(t),
			expectedConverged: true,
		},
		{
			name: "encryption config mismatch",
			deployers: []statemachine.Deployer{
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), true, nil),
				newFakeDeployer(func() *corev1.Secret {
					ec := createDefaultEncryptionConfig()
					ec.Resources = append(ec.Resources, apiserverconfigv1.ResourceConfiguration{Resources: []string{"pods"}})
					return encryptionCfgToSecret(t, ec)
				}(), true, nil),
			},
			expectedSecret:    nil,
			expectedConverged: false,
		},
		{
			name: "deployer2 hasn't converged",
			deployers: []statemachine.Deployer{
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), true, nil),
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), false, nil),
			},
			expectedConverged: false,
		},
		{
			name: "deployer1 reported an error",
			deployers: []statemachine.Deployer{
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), false, fmt.Errorf("nasty error")),
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), true, nil),
			},
			expectedConverged: false,
			expectedErr:       true,
		},
		{
			name: "happy path with a single deployer",
			deployers: []statemachine.Deployer{
				newFakeDeployer(createDefaultSecretWithEncryptionConfig(t), true, nil),
			},
			expectedConverged: true,
			expectedSecret:    createDefaultSecretWithEncryptionConfig(t),
		},
		{
			name:      "no-op when no deployers",
			deployers: []statemachine.Deployer{},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			target := deployer.NewUnionRevisionLabelPodDeployer(scenario.deployers...)

			actualSecret, actualConverged, actualErr := target.DeployedEncryptionConfigSecret()

			if actualErr != nil && !scenario.expectedErr {
				t.Errorf("got unexpected error %v", actualErr)
			}
			if actualErr == nil && scenario.expectedErr {
				t.Error("expected an error but didn't get one")
			}
			if scenario.expectedConverged != actualConverged {
				t.Errorf("expected converged to be %v, got %v", scenario.expectedConverged, actualConverged)
			}
			if !equality.Semantic.DeepEqual(actualSecret, scenario.expectedSecret) {
				t.Error(fmt.Errorf("retruned secret mismatch, diff = %s", diff.ObjectDiff(actualSecret, scenario.expectedSecret)))
			}
		})
	}
}

func createDefaultSecretWithEncryptionConfig(t *testing.T) *corev1.Secret {
	ec := createDefaultEncryptionConfig()
	return encryptionCfgToSecret(t, ec)
}

func encryptionCfgToSecret(t *testing.T, ec *apiserverconfigv1.EncryptionConfiguration) *corev1.Secret {
	s, err := encryptionconfig.ToSecret("targetNs", fmt.Sprintf("%s-%s", "encryption-config", "1"), ec)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func createDefaultEncryptionConfig() *apiserverconfigv1.EncryptionConfiguration {
	keysResForSecrets := encryptiontesting.EncryptionKeysResourceTuple{
		Resource: "secrets",
		Keys: []apiserverconfigv1.Key{
			{
				Name:   "1",
				Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
			},
		},
	}
	keysResForConfigMaps := encryptiontesting.EncryptionKeysResourceTuple{
		Resource: "configmaps",
		Keys: []apiserverconfigv1.Key{
			{
				Name:   "1",
				Secret: "NzFlYTdjOTE0MTlhNjhmZDEyMjRmODhkNTAzMTZiNGU=",
			},
		},
	}

	return encryptiontesting.CreateEncryptionCfgWithWriteKey([]encryptiontesting.EncryptionKeysResourceTuple{keysResForConfigMaps, keysResForSecrets})
}

type fakeDeployer struct {
	secret    *corev1.Secret
	converged bool
	err       error
}

func newFakeDeployer(secret *corev1.Secret, converged bool, err error) *fakeDeployer {
	return &fakeDeployer{secret: secret, converged: converged, err: err}
}

func (d *fakeDeployer) DeployedEncryptionConfigSecret() (secret *corev1.Secret, converged bool, err error) {
	return d.secret, d.converged, d.err
}

func (d *fakeDeployer) AddEventHandler(handler cache.ResourceEventHandler) {
}

func (d *fakeDeployer) HasSynced() bool {
	return true
}
