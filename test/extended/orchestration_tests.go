// +build extended

package extended

import (
	testutil "github.com/openshift/origin/test/util"
	"testing"
)

func init() {
	testutil.RequireServer()
}

// TestPushSecretName exercises one of the complex Build scenarios, where you
// first build a Docker image using Docker build strategy, which will later by
// consumed by Custom build strategy to verify that the 'PushSecretName' (Docker
// credentials) were successfully transported to the builder. The content of the
// Secret file is verified in the end.
func TestReplicationContrillerResize(t *testing.T) {
	namespace := testutil.RandomNamespace("secret")
	kclient, _ := testutil.GetClusterAdminKubeClient(testutil.KubeConfigPath())

	stream := testutil.CreateSampleImageStream(namespace)
	if stream == nil {
		t.Fatal("Failed to create ImageStream")
	}
	defer testutil.DeleteSampleImageStream(stream, namespace)

	// Create Secret with dockercfg
	secret := testutil.GetSecretFixture("fixtures/test-secret.json")
	// TODO: Why do I need to set namespace here?
	secret.Namespace = namespace
	_, err := kclient.Secrets(namespace).Create(secret)
	if err != nil {
		t.Fatalf("Failed to create Secret: %v", err)
	}

}
