package util

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestTrustedMergeEnvWithoutDuplicates(t *testing.T) {
	input := []corev1.EnvVar{
		// stripped by whitelist
		{Name: "foo", Value: "bar"},
		// stripped by whitelist
		{Name: "input", Value: "inputVal"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
		{Name: "BUILD_LOGLEVEL", Value: "source"},
	}
	output := []corev1.EnvVar{
		{Name: "foo", Value: "test"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
	}
	// resolve conflicts w/ input value
	MergeTrustedEnvWithoutDuplicates(input, &output, true)

	if len(output) != 3 {
		t.Errorf("Expected output to contain input items len==3 (%d), %#v", len(output), output)
	}

	if output[0].Name != "foo" {
		t.Errorf("Expected output to have env 'foo', got %+v", output[0])
	}
	if output[0].Value != "test" {
		t.Errorf("Expected output env 'foo' to have value 'test', got %+v", output[0])
	}
	if output[1].Name != "GIT_SSL_NO_VERIFY" {
		t.Errorf("Expected output to have env 'GIT_SSL_NO_VERIFY', got %+v", output[1])
	}
	if output[1].Value != "source" {
		t.Errorf("Expected output env 'GIT_SSL_NO_VERIFY' to have value 'loglevel', got %+v", output[1])
	}
	if output[2].Name != "BUILD_LOGLEVEL" {
		t.Errorf("Expected output to have env 'BUILD_LOGLEVEL', got %+v", output[1])
	}
	if output[2].Value != "source" {
		t.Errorf("Expected output env 'BUILD_LOGLEVEL' to have value 'loglevel', got %+v", output[1])
	}

	input = []corev1.EnvVar{
		// stripped by whitelist
		{Name: "foo", Value: "bar"},
		// stripped by whitelist
		{Name: "input", Value: "inputVal"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
		{Name: "BUILD_LOGLEVEL", Value: "source"},
	}
	output = []corev1.EnvVar{
		{Name: "foo", Value: "test"},
		{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
	}
	// resolve conflicts w/ output value
	MergeTrustedEnvWithoutDuplicates(input, &output, false)

	if len(output) != 3 {
		t.Errorf("Expected output to contain input items len==3 (%d), %#v", len(output), output)
	}

	if output[0].Name != "foo" {
		t.Errorf("Expected output to have env 'foo', got %+v", output[0])
	}
	if output[0].Value != "test" {
		t.Errorf("Expected output env 'foo' to have value 'test', got %+v", output[0])
	}
	if output[1].Name != "GIT_SSL_NO_VERIFY" {
		t.Errorf("Expected output to have env 'GIT_SSL_NO_VERIFY', got %+v", output[1])
	}
	if output[1].Value != "target" {
		t.Errorf("Expected output env 'GIT_SSL_NO_VERIFY' to have value 'loglevel', got %+v", output[1])
	}
	if output[2].Name != "BUILD_LOGLEVEL" {
		t.Errorf("Expected output to have env 'BUILD_LOGLEVEL', got %+v", output[1])
	}
	if output[2].Value != "source" {
		t.Errorf("Expected output env 'BUILD_LOGLEVEL' to have value 'source', got %+v", output[1])
	}

}
