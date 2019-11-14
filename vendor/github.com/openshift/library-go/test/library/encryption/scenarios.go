package encryption

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	configv1 "github.com/openshift/api/config/v1"
)

type BasicScenario struct {
	Namespace                       string
	LabelSelector                   string
	EncryptionConfigSecretName      string
	EncryptionConfigSecretNamespace string
	OperatorNamespace               string
	TargetGRs                       []schema.GroupResource
	AssertFunc                      func(t testing.TB, clientSet ClientSet, expectedMode configv1.EncryptionType, namespace, labelSelector string)
}

func TestEncryptionTypeIdentity(t *testing.T, scenario BasicScenario) {
	e := NewE(t, PrintEventsOnFailure(scenario.OperatorNamespace))
	clientSet := SetAndWaitForEncryptionType(e, configv1.EncryptionTypeIdentity, scenario.TargetGRs, scenario.Namespace, scenario.LabelSelector)
	scenario.AssertFunc(e, clientSet, configv1.EncryptionTypeIdentity, scenario.Namespace, scenario.LabelSelector)
}

func TestEncryptionTypeUnset(t *testing.T, scenario BasicScenario) {
	e := NewE(t, PrintEventsOnFailure(scenario.OperatorNamespace))
	clientSet := SetAndWaitForEncryptionType(e, "", scenario.TargetGRs, scenario.Namespace, scenario.LabelSelector)
	scenario.AssertFunc(e, clientSet, configv1.EncryptionTypeIdentity, scenario.Namespace, scenario.LabelSelector)
}

func TestEncryptionTypeAESCBC(t *testing.T, scenario BasicScenario) {
	e := NewE(t, PrintEventsOnFailure(scenario.OperatorNamespace))
	clientSet := SetAndWaitForEncryptionType(e, configv1.EncryptionTypeAESCBC, scenario.TargetGRs, scenario.Namespace, scenario.LabelSelector)
	scenario.AssertFunc(e, clientSet, configv1.EncryptionTypeAESCBC, scenario.Namespace, scenario.LabelSelector)
	AssertEncryptionConfig(e, clientSet, scenario.EncryptionConfigSecretName, scenario.EncryptionConfigSecretNamespace, scenario.TargetGRs)
}

type OnOffScenario struct {
	BasicScenario
	CreateResourceFunc             func(t testing.TB, clientSet ClientSet, namespace string) runtime.Object
	AssertResourceEncryptedFunc    func(t testing.TB, clientSet ClientSet, resource runtime.Object)
	AssertResourceNotEncryptedFunc func(t testing.TB, clientSet ClientSet, resource runtime.Object)
	ResourceFunc                   func(t testing.TB, namespace string) runtime.Object
	ResourceName                   string
}

func TestEncryptionTurnOnAndOff(t *testing.T, scenario OnOffScenario) {
	scenarios := []struct {
		name     string
		testFunc func(*testing.T)
	}{
		{name: fmt.Sprintf("CreateAndStore%s", scenario.ResourceName), testFunc: func(t *testing.T) {
			e := NewE(t)
			scenario.CreateResourceFunc(e, GetClients(e), scenario.Namespace)
		}},
		{name: "OnAESCBC", testFunc: func(t *testing.T) { TestEncryptionTypeAESCBC(t, scenario.BasicScenario) }},
		{name: fmt.Sprintf("Assert%sEncrypted", scenario.ResourceName), testFunc: func(t *testing.T) {
			e := NewE(t)
			scenario.AssertResourceEncryptedFunc(e, GetClients(e), scenario.ResourceFunc(e, scenario.Namespace))
		}},
		{name: "OffIdentity", testFunc: func(t *testing.T) { TestEncryptionTypeIdentity(t, scenario.BasicScenario) }},
		{name: fmt.Sprintf("Assert%sNotEncrypted", scenario.ResourceName), testFunc: func(t *testing.T) {
			e := NewE(t)
			scenario.AssertResourceNotEncryptedFunc(e, GetClients(e), scenario.ResourceFunc(e, scenario.Namespace))
		}},
		{name: "OnAESCBCSecond", testFunc: func(t *testing.T) { TestEncryptionTypeAESCBC(t, scenario.BasicScenario) }},
		{name: fmt.Sprintf("Assert%sEncryptedSecond", scenario.ResourceName), testFunc: func(t *testing.T) {
			e := NewE(t)
			scenario.AssertResourceEncryptedFunc(e, GetClients(e), scenario.ResourceFunc(e, scenario.Namespace))
		}},
		{name: "OffIdentitySecond", testFunc: func(t *testing.T) { TestEncryptionTypeIdentity(t, scenario.BasicScenario) }},
		{name: fmt.Sprintf("Assert%sNotEncryptedSecond", scenario.ResourceName), testFunc: func(t *testing.T) {
			e := NewE(t)
			scenario.AssertResourceNotEncryptedFunc(e, GetClients(e), scenario.ResourceFunc(e, scenario.Namespace))
		}},
	}

	// run scenarios
	for _, testScenario := range scenarios {
		t.Run(testScenario.name, testScenario.testFunc)
		if t.Failed() {
			t.Errorf("stopping the test as %q scenario failed", testScenario.name)
			return
		}
	}
}

type RotationScenario struct {
	BasicScenario
	CreateResourceFunc    func(t testing.TB, clientSet ClientSet, namespace string) runtime.Object
	GetRawResourceFunc    func(t testing.TB, clientSet ClientSet, namespace string) string
	UnsupportedConfigFunc UpdateUnsupportedConfigFunc
}

// TestEncryptionRotation first encrypts data with aescbc key
// then it forces a key rotation by setting the "encyrption.Reason" in the operator's configuration file
func TestEncryptionRotation(t *testing.T, scenario RotationScenario) {
	// test data
	ns := scenario.Namespace
	labelSelector := scenario.LabelSelector

	// step 1: create the desired resource
	e := NewE(t)
	clientSet := GetClients(e)
	scenario.CreateResourceFunc(e, GetClients(e), ns)

	// step 2: run encryption aescbc scenario
	TestEncryptionTypeAESCBC(t, scenario.BasicScenario)

	// step 3: take samples
	rawEncryptedResourceWithKey1 := scenario.GetRawResourceFunc(e, clientSet, ns)

	// step 4: force key rotation and wait for migration to complete
	lastMigratedKeyMeta, err := GetLastKeyMeta(clientSet.Kube, ns, labelSelector)
	require.NoError(e, err)
	require.NoError(e, ForceKeyRotation(e, scenario.UnsupportedConfigFunc, fmt.Sprintf("test-key-rotation-%s", rand.String(4))))
	WaitForNextMigratedKey(e, clientSet.Kube, lastMigratedKeyMeta, scenario.TargetGRs, ns, labelSelector)
	scenario.AssertFunc(e, clientSet, configv1.EncryptionTypeAESCBC, ns, labelSelector)

	// step 5: verify if the provided resource was encrypted with a different key (step 2 vs step 4)
	rawEncryptedResourceWithKey2 := scenario.GetRawResourceFunc(e, clientSet, ns)
	if rawEncryptedResourceWithKey1 == rawEncryptedResourceWithKey2 {
		t.Errorf("expected the resource to has a different content after a key rotation,\ncontentBeforeRotation %s\ncontentAfterRotation %s", rawEncryptedResourceWithKey1, rawEncryptedResourceWithKey2)
	}

	// TODO: assert conditions - operator and encryption migration controller must report status as active not progressing, and not failing for all scenarios
}
