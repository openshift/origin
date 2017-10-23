package example

/* Used to generate output for testing

import (
	"fmt"
	"testing"
)

func TestSubTestWithFailures(t *testing.T) {
	t.Run("subtest-pass-1", func(t *testing.T) {})
	t.Run("subtest-pass-2", func(t *testing.T) {})
	t.Run("subtest-fail-1", func(t *testing.T) { fmt.Printf("text line\n"); t.Logf("log line"); t.Errorf("failed") })
}

func TestSubTestWithFirstFailures(t *testing.T) {
	t.Run("subtest-fail-1", func(t *testing.T) { fmt.Printf("text line\n"); t.Logf("log line"); t.Errorf("failed") })
	t.Run("subtest-pass-1", func(t *testing.T) {})
	t.Run("subtest-pass-2", func(t *testing.T) {})
}

func TestSubTestWithSubTestFailures(t *testing.T) {
	t.Run("subtest-pass-1", func(t *testing.T) {})
	t.Run("subtest-pass-2", func(t *testing.T) {})
	t.Run("subtest-fail-1", func(t *testing.T) {
		fmt.Printf("text line\n")
		t.Logf("log line before")
		t.Run("sub-subtest-pass-1", func(t *testing.T) {})
		t.Run("sub-subtest-pass-2", func(t *testing.T) {})
		t.Run("sub-subtest-fail-1", func(t *testing.T) { fmt.Printf("text line\n"); t.Logf("log line"); t.Errorf("failed") })
		t.Logf("log line after")
	})
}

func TestSubTestWithMiddleFailures(t *testing.T) {
	t.Run("subtest-pass-1", func(t *testing.T) {})
	t.Run("subtest-fail-1", func(t *testing.T) { fmt.Printf("text line\n"); t.Logf("log line"); t.Errorf("failed") })
	t.Run("subtest-pass-2", func(t *testing.T) {})
}
*/
