package controllers

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
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
