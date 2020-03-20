package encryption

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/stretchr/testify/require"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/client-go/kubernetes"
)

var protoEncodingPrefix = []byte{0x6b, 0x38, 0x73, 0x00}

var (
	apiserverScheme = runtime.NewScheme()
	apiserverCodecs = serializer.NewCodecFactory(apiserverScheme)
)

const (
	jsonEncodingPrefix           = "{"
	protoEncryptedDataPrefix     = "k8s:enc:"
	aesCBCTransformerPrefixV1    = "k8s:enc:aescbc:v1:"
	secretboxTransformerPrefixV1 = "k8s:enc:secretbox:v1:"
)

func init() {
	utilruntime.Must(apiserverconfigv1.AddToScheme(apiserverScheme))
}

// AssertEncryptionConfig checks if the encryption config holds only targetGRs, this ensures that only those resources were encrypted,
// we don't check the keys because e2e tests are run randomly and we would have to consider all encryption secrets to get the right order of the keys.
// We test the content of the encryption config in more detail in unit and integration tests
func AssertEncryptionConfig(t testing.TB, clientSet ClientSet, encryptionConfigSecretName string, namespace string, targetGRs []schema.GroupResource) {
	t.Helper()
	t.Logf("Checking if %q in %q has desired GRs %v", encryptionConfigSecretName, namespace, targetGRs)
	encryptionCofnigSecret, err := clientSet.Kube.CoreV1().Secrets(namespace).Get(context.TODO(), encryptionConfigSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	encodedEncryptionConfig, foundEncryptionConfig := encryptionCofnigSecret.Data["encryption-config"]
	if !foundEncryptionConfig {
		t.Errorf("Haven't found encryption config at %q key in the encryption secret %q", "encryption-config", encryptionConfigSecretName)
	}

	decoder := apiserverCodecs.UniversalDecoder(apiserverconfigv1.SchemeGroupVersion)
	encryptionConfigObj, err := runtime.Decode(decoder, encodedEncryptionConfig)
	require.NoError(t, err)
	encryptionConfig, ok := encryptionConfigObj.(*apiserverconfigv1.EncryptionConfiguration)
	if !ok {
		t.Errorf("Unable to decode encryption config, unexpected wrong type %T", encryptionConfigObj)
	}

	for _, rawActualResource := range encryptionConfig.Resources {
		if len(rawActualResource.Resources) != 1 {
			t.Errorf("Invalid encryption config for resource %s, expected exactly one resource, got %d", rawActualResource.Resources, len(rawActualResource.Resources))
		}
		actualResource := schema.ParseGroupResource(rawActualResource.Resources[0])
		actualResourceFound := false
		for _, expectedResource := range targetGRs {
			if reflect.DeepEqual(expectedResource, actualResource) {
				actualResourceFound = true
				break
			}
		}
		if !actualResourceFound {
			t.Errorf("Encryption config has an invalid resource %v", actualResource)
		}
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

func VerifyResources(t testing.TB, etcdClient EtcdClient, etcdKeyPreifx string, expectedMode string, allowEmpty bool) (int, error) {
	timeout, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	resp, err := etcdClient.Get(timeout, etcdKeyPreifx, clientv3.WithPrefix())
	switch {
	case err != nil:
		return 0, fmt.Errorf("failed to list prefix %s: %v", etcdKeyPreifx, err)
	case (resp.Count == 0 || len(resp.Kvs) == 0) && !allowEmpty:
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
