package factory

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/controller"
)

func TestControllerRetryFunc(t *testing.T) {
	obj := &kapi.Pod{}
	obj.Name = "testpod"
	obj.Namespace = "testNS"

	testErr := fmt.Errorf("test error")
	tests := []struct {
		name       string
		retryCount int
		isFatal    func(err error) bool
		err        error
		expect     bool
	}{
		{
			name:       "maxRetries-1 retries",
			retryCount: maxRetries - 1,
			err:        testErr,
			expect:     true,
		},
		{
			name:       "maxRetries+1 retries",
			retryCount: maxRetries + 1,
			err:        testErr,
			expect:     false,
		},
		{
			name:       "isFatal returns true",
			retryCount: 0,
			err:        testErr,
			isFatal: func(err error) bool {
				if err != testErr {
					t.Errorf("Unexpected error: %v", err)
				}
				return true
			},
			expect: false,
		},
		{
			name:       "isFatal returns false",
			retryCount: 0,
			err:        testErr,
			isFatal: func(err error) bool {
				if err != testErr {
					t.Errorf("Unexpected error: %v", err)
				}
				return false
			},
			expect: true,
		},
	}

	for _, tc := range tests {
		f := retryFunc("test kind", tc.isFatal)
		result := f(obj, tc.err, controller.Retry{Count: tc.retryCount})
		if result != tc.expect {
			t.Errorf("%s: unexpected result. Expected: %v. Got: %v", tc.name, tc.expect, result)
		}
	}
}
