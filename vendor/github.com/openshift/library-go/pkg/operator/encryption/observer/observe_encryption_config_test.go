package observer

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corelistersv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/encryption/encryptionconfig"
	encryptiontesting "github.com/openshift/library-go/pkg/operator/encryption/testing"
	"github.com/openshift/library-go/pkg/operator/events"
)

type secretsListers struct {
	configobserver.Listers

	secretLister_ corelistersv1.SecretLister
}

func (l secretsListers) SecretLister() corelistersv1.SecretLister {
	return l.secretLister_
}

func TestEncryptionConfigObserver(t *testing.T) {
	scenarios := []struct {
		name             string
		input            map[string]interface{}
		initialResources []runtime.Object

		expectedOutput map[string]interface{}
		expectedEvents []*corev1.Event
	}{
		// scenario 1
		{
			name: "a secret with encryption config exits thus encryption-provider-config flag is set",
			initialResources: func() []runtime.Object {
				ret := []runtime.Object{}
				ec := encryptiontesting.CreateEncryptionCfgNoWriteKey("1", "NjFkZWY5NjRmYjk2N2Y1ZDdjNDRhMmFmOGRhYjY4NjU=", "secrets")
				ecs, err := encryptionconfig.ToSecret("kms", "encryption-config", ec)
				if err != nil {
					t.Fatal(err)
				}
				ret = append(ret, ecs)
				return ret
			}(),
			expectedOutput: func() map[string]interface{} {
				ret := map[string]interface{}{}
				ret["apiServerArguments"] = map[string]interface{}{
					"encryption-provider-config": []interface{}{"/etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config"},
				}
				return ret
			}(),
			expectedEvents: []*corev1.Event{
				{Reason: "ObserveEncryptionConfigChanged", Message: "encryption config file changed from [] to /etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config"},
			},
		},

		// scenario 2
		{
			name:           "no secret with encryption config exits thus no encryption-provider-config flag is set",
			expectedOutput: map[string]interface{}{},
			expectedEvents: []*corev1.Event{}, // we expect no events
		},

		// scenario 3
		{
			name: "encryption-provider-config flag was set in the past but the secret with encryption config is missing",
			input: func() map[string]interface{} {
				ret := map[string]interface{}{}
				ret["apiServerArguments"] = map[string]interface{}{
					"encryption-provider-config": []interface{}{"/etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config"},
				}
				return ret
			}(),
			expectedOutput: func() map[string]interface{} {
				ret := map[string]interface{}{}
				ret["apiServerArguments"] = map[string]interface{}{
					"encryption-provider-config": []interface{}{"/etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config"},
				}
				return ret
			}(),
			expectedEvents: []*corev1.Event{
				{Reason: "ObserveEncryptionConfigNotFound", Message: "encryption config secret kms/encryption-config not found after encryption has been enabled"},
			},
		},

		// scenario 4
		{
			name: "warn about encryption-provider-config value change",
			initialResources: func() []runtime.Object {
				ret := []runtime.Object{}
				ec := encryptiontesting.CreateEncryptionCfgNoWriteKey("1", "NjFkZWY5NjRmYjk2N2Y1ZDdjNDRhMmFmOGRhYjY4NjU=", "secrets")
				ecs, err := encryptionconfig.ToSecret("kms", "encryption-config", ec)
				if err != nil {
					t.Fatal(err)
				}
				ret = append(ret, ecs)
				return ret
			}(),
			input: func() map[string]interface{} {
				ret := map[string]interface{}{}
				ret["apiServerArguments"] = map[string]interface{}{
					"encryption-provider-config": []interface{}{"some_path"},
				}
				return ret
			}(),
			expectedOutput: func() map[string]interface{} {
				ret := map[string]interface{}{}
				ret["apiServerArguments"] = map[string]interface{}{
					"encryption-provider-config": []interface{}{"/etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config"},
				}
				return ret
			}(),
			expectedEvents: []*corev1.Event{
				{Reason: "ObserveEncryptionConfigChanged", Message: "encryption config file changed from [some_path] to /etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config"},
			},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			listers := secretsListers{}
			{
				indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
				for _, obj := range scenario.initialResources {
					err := indexer.Add(obj)
					if err != nil {
						t.Fatal(err)
					}
				}
				listers.secretLister_ = corelistersv1.NewSecretLister(indexer)
			}
			eventRec := events.NewInMemoryRecorder("encryption-config-observer")

			target := NewEncryptionConfigObserver("kms", "/etc/kubernetes/static-pod-resources/secrets/encryption-config/encryption-config")
			result, err := target(listers, eventRec, scenario.input)
			if err != nil {
				t.Fatal(err)
			}

			if !cmp.Equal(result, scenario.expectedOutput) {
				t.Fatal(fmt.Errorf("%s", cmp.Diff(result, scenario.expectedOutput)))
			}

			// validate events
			{
				recordedEvents := eventRec.Events()
				if len(scenario.expectedEvents) != len(recordedEvents) {
					t.Fatalf("expected to observe %d events but got %d", len(scenario.expectedEvents), len(recordedEvents))
				}

				for _, recordedEvent := range recordedEvents {
					expectedEvent := recordedEvent.DeepCopy()
					recordedEventFound := false

					for _, expectedEventShort := range scenario.expectedEvents {
						expectedEvent.Message = expectedEventShort.Message
						expectedEvent.Reason = expectedEventShort.Reason
						if cmp.Equal(expectedEvent, recordedEvent) {
							recordedEventFound = true
							break
						}
					}

					if !recordedEventFound {
						t.Fatalf("expected event with reason = %q and message %q wasn't found\n recorded events = %v", expectedEvent.Reason, expectedEvent.Message, recordedEvents)
					}
				}
			}
		})
	}
}
