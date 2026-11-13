package registryauth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	clientgotesting "k8s.io/client-go/testing"
)

const testDockerConfig = `{"auths":{"registry.ci.openshift.org":{"auth":"dGVzdA=="}}}`

// clusterWithPullSecret returns a client for a cluster holding the usual
// openshift-config/pull-secret.
func clusterWithPullSecret() corev1client.CoreV1Interface {
	return fake.NewSimpleClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "pull-secret", Namespace: "openshift-config"},
		Data:       map[string][]byte{".dockerconfigjson": []byte(testDockerConfig)},
	}).CoreV1()
}

// staticClient returns a ClientFunc yielding the given client and error.
func staticClient(client corev1client.CoreV1Interface, err error) ClientFunc {
	return func() (corev1client.CoreV1Interface, error) {
		return client, err
	}
}

// forbiddenClient returns a ClientFunc that fails the test when called.
func forbiddenClient(t *testing.T) ClientFunc {
	t.Helper()
	return func() (corev1client.CoreV1Interface, error) {
		t.Error("cluster client was created, but no credential source should have needed it")
		return nil, nil
	}
}

func TestDetermineAuthFilePath(t *testing.T) {
	tests := []struct {
		name string
		// registryAuthFile is the value of $REGISTRY_AUTH_FILE.
		registryAuthFile string
		// ciProfileExists controls whether the cluster profile pull secret exists.
		ciProfileExists bool
		// ciProfilePath overrides the location checked for the cluster profile
		// pull secret. It defaults to a nonexistent file in a temporary directory.
		ciProfilePath func(t *testing.T) string
		client        func(t *testing.T) ClientFunc
		want          func(t *testing.T, got, tmpDir, ciProfilePath string)
		wantErr       string
	}{
		{
			name:             "REGISTRY_AUTH_FILE is used when set",
			registryAuthFile: "/var/run/my-auth.json",
			ciProfileExists:  true,
			client:           forbiddenClient,
			want: func(t *testing.T, got, _, _ string) {
				assertPath(t, got, "/var/run/my-auth.json")
			},
		},
		{
			name:            "cluster profile pull secret is used when the env var is unset",
			ciProfileExists: true,
			client:          forbiddenClient,
			want: func(t *testing.T, got, _, ciProfilePath string) {
				assertPath(t, got, ciProfilePath)
			},
		},
		{
			name:   "cluster pull secret is copied into tmpDir when nothing else is available",
			client: func(*testing.T) ClientFunc { return staticClient(clusterWithPullSecret(), nil) },
			want: func(t *testing.T, got, tmpDir, _ string) {
				if dir := filepath.Dir(got); dir != tmpDir {
					t.Errorf("expected auth file in %q, got %q", tmpDir, got)
				}
				if base := filepath.Base(got); !strings.HasPrefix(base, ".dockerconfigjson") {
					t.Errorf("expected auth file name to start with %q, got %q", ".dockerconfigjson", base)
				}
				assertAuthFile(t, got, testDockerConfig)
			},
		},
		{
			name:   "anonymous access when the cluster has no pull secret",
			client: func(*testing.T) ClientFunc { return staticClient(fake.NewSimpleClientset().CoreV1(), nil) },
			want: func(t *testing.T, got, _, _ string) {
				assertPath(t, got, "")
			},
		},
		{
			name:   "anonymous access when no ClientFunc is provided",
			client: func(*testing.T) ClientFunc { return nil },
			want: func(t *testing.T, got, _, _ string) {
				assertPath(t, got, "")
			},
		},
		{
			name:   "anonymous access when the ClientFunc yields no client",
			client: func(*testing.T) ClientFunc { return staticClient(nil, nil) },
			want: func(t *testing.T, got, _, _ string) {
				assertPath(t, got, "")
			},
		},
		{
			name: "unreadable cluster profile location falls through to the next source",
			ciProfilePath: func(t *testing.T) string {
				// A path below a regular file cannot be stat'ed, which is an
				// error other than "not exist".
				notADir := filepath.Join(t.TempDir(), "not-a-dir")
				if err := os.WriteFile(notADir, nil, 0600); err != nil {
					t.Fatalf("failed to write %q: %v", notADir, err)
				}
				return filepath.Join(notADir, "pull-secret")
			},
			client: func(*testing.T) ClientFunc { return nil },
			want: func(t *testing.T, got, _, _ string) {
				assertPath(t, got, "")
			},
		},
		{
			name: "unexpected error reading the cluster pull secret is reported",
			client: func(*testing.T) ClientFunc {
				clientset := fake.NewSimpleClientset()
				clientset.PrependReactor("get", "secrets", func(clientgotesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("connection refused")
				})
				return staticClient(clientset.CoreV1(), nil)
			},
			wantErr: "unable to read ephemeral cluster pull secret",
		},
		{
			name: "error creating the cluster client is reported",
			client: func(*testing.T) ClientFunc {
				return staticClient(nil, errors.New("no kubeconfig"))
			},
			wantErr: "unable to create client for the cluster under test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REGISTRY_AUTH_FILE", tt.registryAuthFile)

			tmpDir := t.TempDir()
			ciProfilePath := filepath.Join(t.TempDir(), "pull-secret")
			if tt.ciProfilePath != nil {
				ciProfilePath = tt.ciProfilePath(t)
			}
			if tt.ciProfileExists {
				if err := os.WriteFile(ciProfilePath, []byte(testDockerConfig), 0600); err != nil {
					t.Fatalf("failed to write cluster profile pull secret: %v", err)
				}
			}

			got, err := determineAuthFilePath(context.Background(), tt.client(t), tmpDir, ciProfilePath)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.want(t, got, tmpDir, ciProfilePath)
		})
	}
}

// TestDetermineAuthFilePathDoesNotReuseExistingFile checks that the auth file is
// always created exclusively. Writing over a predictable name would let a
// pre-existing file in tmpDir keep its wider permissions.
func TestDetermineAuthFilePathDoesNotReuseExistingFile(t *testing.T) {
	t.Setenv("REGISTRY_AUTH_FILE", "")
	tmpDir := t.TempDir()
	ciProfilePath := filepath.Join(t.TempDir(), "pull-secret")

	const staleContent = `{"auths":{}}`
	stalePath := filepath.Join(tmpDir, ".dockerconfigjson")
	if err := os.WriteFile(stalePath, []byte(staleContent), 0600); err != nil {
		t.Fatalf("failed to write pre-existing auth file: %v", err)
	}
	// Chmod explicitly so the file is world-readable regardless of umask.
	if err := os.Chmod(stalePath, 0644); err != nil {
		t.Fatalf("failed to set permissions on pre-existing auth file: %v", err)
	}

	got, err := determineAuthFilePath(context.Background(), staticClient(clusterWithPullSecret(), nil), tmpDir, ciProfilePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == stalePath {
		t.Fatalf("auth file was written to the pre-existing path %q", stalePath)
	}
	assertAuthFile(t, got, testDockerConfig)

	staleAfter, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("failed to read pre-existing auth file: %v", err)
	}
	if string(staleAfter) != staleContent {
		t.Errorf("pre-existing file was modified: expected %q, got %q", staleContent, string(staleAfter))
	}
}

// assertPath fails the test unless got equals want.
func assertPath(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("expected auth file path %q, got %q", want, got)
	}
}

// assertAuthFile checks that path holds wantContent and is readable only by its
// owner.
func assertAuthFile(t *testing.T, path, wantContent string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected auth file at %q: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("expected auth file mode 0600, got %04o", perm)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read auth file: %v", err)
	}
	if string(content) != wantContent {
		t.Errorf("expected auth file to contain %q, got %q", wantContent, string(content))
	}
}
