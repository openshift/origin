package util

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const validKubeconfig = `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`

func TestGetClientConfigRetryOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	if err := os.WriteFile(path, []byte(validKubeconfig), 0644); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(200 * time.Millisecond)
		os.Remove(path)
		time.Sleep(800 * time.Millisecond)
		os.WriteFile(path, []byte(validKubeconfig), 0644)
	}()

	time.Sleep(300 * time.Millisecond)

	cfg, err := GetClientConfig(path)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if cfg.Host != "https://localhost:6443" {
		t.Fatalf("unexpected host: %s", cfg.Host)
	}
}

func TestGetClientConfigRetryOnEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(500 * time.Millisecond)
		os.WriteFile(path, []byte(validKubeconfig), 0644)
	}()

	cfg, err := GetClientConfig(path)
	if err != nil {
		t.Fatalf("expected retry to succeed, got error: %v", err)
	}
	if cfg.Host != "https://localhost:6443" {
		t.Fatalf("unexpected host: %s", cfg.Host)
	}
}

func TestGetClientConfigFailsOnPermanentMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "kubeconfig")

	start := time.Now()
	_, err := GetClientConfig(path)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for permanently missing file")
	}
	if elapsed > 15*time.Second {
		t.Fatalf("took too long: %v", elapsed)
	}
}

func TestGetClientConfigSucceedsImmediately(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kubeconfig")

	if err := os.WriteFile(path, []byte(validKubeconfig), 0644); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	cfg, err := GetClientConfig(path)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "https://localhost:6443" {
		t.Fatalf("unexpected host: %s", cfg.Host)
	}
	if elapsed > 1*time.Second {
		t.Fatalf("should have succeeded immediately, took %v", elapsed)
	}
}
