package installerpod

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func addFakeSecret(name string) *v1.Secret {
	secret := &v1.Secret{}
	secret.Name = name
	secret.Namespace = "test"
	return secret
}

func TestGetSecretWithRetry(t *testing.T) {
	tests := []struct {
		name              string
		getSecretName     string
		secrets           []runtime.Object
		optional          bool
		expectErr         bool
		expectRetries     bool
		sendInternalError bool
	}{
		{
			name:          "required secret exists",
			secrets:       []runtime.Object{addFakeSecret("test-secret")},
			getSecretName: "test-secret",
		},
		{
			name:          "optional secret does not exists and we not expect error",
			secrets:       []runtime.Object{},
			getSecretName: "test-secret",
			optional:      true,
			expectRetries: false,
			expectErr:     false,
		},
		{
			name:          "required secret does not exists and no retry on not found error",
			secrets:       []runtime.Object{},
			getSecretName: "test-secret",
			expectRetries: false,
			expectErr:     true,
		},
		{
			name:              "required secret exists and we retry on internal error",
			secrets:           []runtime.Object{addFakeSecret("test-secret")},
			getSecretName:     "test-secret",
			expectRetries:     true,
			sendInternalError: true,
			expectErr:         false,
		},
		{
			name:              "optional secret does not exists and we not retry on internal error",
			secrets:           []runtime.Object{},
			getSecretName:     "test-secret",
			optional:          true,
			sendInternalError: true,
			expectRetries:     true,
			expectErr:         false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(test.secrets...)
			ctx := context.TODO()
			internalErrorChan := make(chan struct{})

			client.PrependReactor("get", "secrets", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				getAction := action.(ktesting.GetAction)
				if getAction.GetName() != test.getSecretName {
					return false, nil, nil
				}

				// Send 500 error. Closing the channel means we remove this reactor from the reaction chain.
				if test.sendInternalError {
					close(internalErrorChan)
					return true, nil, errors.NewInternalError(fmt.Errorf("test"))
				} else {
					return false, nil, nil
				}
			})
			timeoutContext, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			ctx = timeoutContext

			options := &InstallOptions{KubeClient: client, Namespace: "test"}

			// If we have test that send internal error, wait for the internal error to be send and then remove the
			// reactor immediately. This should cause the client to retry and we observe that retries in actions.
			if test.sendInternalError {
				go func(c *fake.Clientset) {
					<-internalErrorChan
					c.Lock()
					defer c.Unlock()
					c.ReactionChain = c.ReactionChain[1 : len(c.ReactionChain)-1]
				}(client)
			}

			_, err := options.getSecretWithRetry(ctx, test.getSecretName, test.optional)
			switch {
			case err != nil && !test.expectErr:
				t.Errorf("unexpected error: %v", err)
				return
			case err == nil && test.expectErr:
				t.Errorf("expected error, got none")
				return
			}

			// -1 means that we get 0 if we only got 1 request (which is ok if we don't expect any retries)
			retries := -1
			for _, action := range client.Actions() {
				if action.GetVerb() != "get" || action.GetResource().Resource != "secrets" {
					continue
				}
				retries++
			}
			switch {
			case retries > 0 && !test.expectRetries:
				t.Errorf("expected no retries, but got %d retries", retries)
			case retries == 0 && test.expectRetries:
				t.Error("expected retries, but got none")
			}
		})
	}
}
