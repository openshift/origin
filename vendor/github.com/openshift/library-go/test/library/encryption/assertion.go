package encryption

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

var protoEncodingPrefix = []byte{0x6b, 0x38, 0x73, 0x00}

const (
	jsonEncodingPrefix           = "{"
	protoEncryptedDataPrefix     = "k8s:enc:"
	aesCBCTransformerPrefixV1    = "k8s:enc:aescbc:v1:"
	secretboxTransformerPrefixV1 = "k8s:enc:secretbox:v1:"
)

func AssertSecretOfLifeNotEncrypted(t testing.TB, clientSet ClientSet, secretOfLife *corev1.Secret) {
	t.Helper()
	rawSecretValue := GetRawSecretOfLife(t, clientSet, secretOfLife.Namespace)
	if !strings.Contains(rawSecretValue, string(secretOfLife.Data["quote"])) {
		t.Errorf("The secret received from etcd doesn't have %q, content of the secret (etcd) %s", string(secretOfLife.Data["quote"]), rawSecretValue)
	}
}

func AssertSecretOfLifeEncrypted(t testing.TB, clientSet ClientSet, secretOfLife *corev1.Secret) {
	t.Helper()
	rawSecretValue := GetRawSecretOfLife(t, clientSet, secretOfLife.Namespace)
	if strings.Contains(rawSecretValue, string(secretOfLife.Data["quote"])) {
		t.Errorf("The secret received from etcd have %q (plain text), content of the secret (etcd) %s", string(secretOfLife.Data["quote"]), rawSecretValue)
	}
}

func AssertLastMigratedKey(t testing.TB, kubeClient kubernetes.Interface, targetGRs []schema.GroupResource, namespace, labelSelector string) {
	t.Helper()
	expectedGRs := targetGRs
	t.Logf("Checking if the last migrated key was used to encrypt %v", expectedGRs)
	lastMigratedKeyMeta, err := GetLastKeyMeta(kubeClient, namespace, labelSelector)
	require.NoError(t, err)
	if len(lastMigratedKeyMeta.Name) == 0 {
		t.Log("Nothing to check no new key was created")
		return
	}

	if len(expectedGRs) != len(lastMigratedKeyMeta.Migrated) {
		t.Errorf("Wrong number of migrated resources for %q key, expected %d, got %d", lastMigratedKeyMeta.Name, len(expectedGRs), len(lastMigratedKeyMeta.Migrated))
	}

	for _, expectedGR := range expectedGRs {
		if !hasResource(expectedGR, lastMigratedKeyMeta.Migrated) {
			t.Errorf("%q wasn't used to encrypt %v, only %v", lastMigratedKeyMeta.Name, expectedGR, lastMigratedKeyMeta.Migrated)
		}
	}
}

func VerifyResources(t testing.TB, etcdClient EtcdClient, etcdKeyPreifx, expectedMode string) (int, error) {
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resp, err := etcdClient.Get(timeout, etcdKeyPreifx, clientv3.WithPrefix())
	switch {
	case err != nil:
		return 0, fmt.Errorf("failed to list prefix %s: %v", etcdKeyPreifx, err)
	case resp.Count == 0 || len(resp.Kvs) == 0:
		return 0, fmt.Errorf("empty list response for prefix %s: %+v", etcdKeyPreifx, resp)
	case resp.More:
		return 0, fmt.Errorf("incomplete list response for prefix %s: %+v", etcdKeyPreifx, resp)
	}

	for _, keyValue := range resp.Kvs {
		if err := verifyPrefixForRawData(expectedMode, keyValue.Value); err != nil {
			return 0, fmt.Errorf("key %s failed check: %v\n%s", keyValue.Key, err, hex.Dump(keyValue.Value))
		}
	}

	return len(resp.Kvs), nil
}

func verifyPrefixForRawData(expectedMode string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	conditionToStr := func(condition bool) string {
		if condition {
			return "encrypted"
		}
		return "unencrypted"
	}

	expectedEncrypted := true
	if expectedMode == "identity" {
		expectedMode = "identity-proto"
		expectedEncrypted = false
	}

	actualMode, isEncrypted := encryptionModeFromEtcdValue(data)
	if expectedEncrypted != isEncrypted {
		return fmt.Errorf("unexpected encrypted state, expected data to be %q but was %q with mode %q", conditionToStr(expectedEncrypted), conditionToStr(isEncrypted), actualMode)
	}
	if actualMode != expectedMode {
		return fmt.Errorf("unexpected encryption mode %q, expected %q, was data encrypted/decrypted with a wrong key", actualMode, expectedMode)
	}

	return nil
}

func encryptionModeFromEtcdValue(data []byte) (string, bool) {
	isEncrypted := bytes.HasPrefix(data, []byte(protoEncryptedDataPrefix)) // all encrypted data has this prefix
	return func() string {
		switch {
		case hasPrefixAndTrailingData(data, []byte(aesCBCTransformerPrefixV1)): // AES-CBC has this prefix
			return "aescbc"
		case hasPrefixAndTrailingData(data, []byte(secretboxTransformerPrefixV1)): // Secretbox has this prefix
			return "secretbox"
		case hasPrefixAndTrailingData(data, []byte(jsonEncodingPrefix)): // unencrypted json data has this prefix
			return "identity-json"
		case hasPrefixAndTrailingData(data, protoEncodingPrefix): // unencrypted protobuf data has this prefix
			return "identity-proto"
		default:
			return "unknown" // this should never happen
		}
	}(), isEncrypted
}

func hasPrefixAndTrailingData(data, prefix []byte) bool {
	return bytes.HasPrefix(data, prefix) && len(data) > len(prefix)
}
