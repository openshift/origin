package controllers

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"

	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
)

func createEncryptionCfgSecret(t *testing.T, targetNs string, revision string, encryptionCfg *apiserverconfigv1.EncryptionConfiguration) *corev1.Secret {
	t.Helper()

	s, err := encryptionconfig.ToSecret(targetNs, fmt.Sprintf("%s-%s", "encryption-config", revision), encryptionCfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

type testProvider struct {
	encryptedGRs []schema.GroupResource
}

func newTestProvider(encryptedGRs []schema.GroupResource) Provider {
	return &testProvider{encryptedGRs: encryptedGRs}
}

func (p *testProvider) EncryptedGRs() []schema.GroupResource {
	return p.encryptedGRs
}

func (p *testProvider) ShouldRunEncryptionControllers() (bool, error) {
	return true, nil
}
