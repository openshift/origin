// Package registryauth resolves the credentials needed to read a release
// payload and the component images it references from an image registry.
//
// Tests and extension binaries frequently extract content from the payload
// under test: openshift-tests extracts OTE extension binaries from component
// images, and suites in other repositories shell out to `oc image extract`.
// Those pulls only succeed with credentials, and which credentials exist
// depends entirely on where the process runs, so this package implements the
// same ladder of credential sources that openshift-tests uses.
package registryauth

import (
	"context"
	"errors"
	"fmt"
	"os"

	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

// CIProfilePullSecretPath is the conventional location of the cluster
// profile's pull secret inside a CI job pod.
const CIProfilePullSecretPath = "/run/secrets/ci.openshift.io/cluster-profile/pull-secret"

// ClientFunc returns a client for the cluster under test. It is only called
// when the cluster pull secret is the last remaining credential source, so
// callers are free to create the client lazily: a caller with no cluster, or
// whose client creation fails hard when no kubeconfig is available, does not
// have to build one up front. A nil ClientFunc, or one returning a nil client,
// skips that source.
type ClientFunc func() (corev1client.CoreV1Interface, error)

// DetermineAuthFilePath returns the path of a registry auth file (a
// ".dockerconfigjson" pull secret) able to read the release payload under test
// and the component images it references, or "" when no credentials are
// available and the caller should use anonymous access. tmpDir is where an
// auth file is written when the credentials have to be copied out of the
// cluster, so it must exist; the returned path is only valid while tmpDir is.
//
// Sources are tried in this order:
//
//  1. $REGISTRY_AUTH_FILE, if set: an auth file with pull secrets capable of
//     reading the payload and its component images.
//  2. CIProfilePullSecretPath, if it exists: the cluster profile's pull secret
//     as mounted by CI.
//  3. Secret openshift-config/pull-secret of the cluster under test, if it
//     exists (MicroShift has none). client is only called to reach this
//     source, and nil skips it.
//
// The ladder exists because the component images a payload references are
// private unless the payload lives in a ci-op-* imagestream, while the payload
// itself may be referenced as: a build farm ci-op-* namespace/imagestream
// (anonymous access permitted), a quay.io/openshift-release-dev location for
// promoted ART payloads (anonymous access permitted), or a
// registry.ci.openshift.org/ocp-<arch>/release:<tag> pull spec (requires a
// registry.ci.openshift.org token).
func DetermineAuthFilePath(ctx context.Context, client ClientFunc, tmpDir string) (string, error) {
	return determineAuthFilePath(ctx, client, tmpDir, CIProfilePullSecretPath)
}

// determineAuthFilePath implements DetermineAuthFilePath with the cluster
// profile pull secret path injectable so the ladder can be tested.
func determineAuthFilePath(ctx context.Context, client ClientFunc, tmpDir, ciProfilePullSecretPath string) (string, error) {
	if registryAuthFilePath := os.Getenv("REGISTRY_AUTH_FILE"); registryAuthFilePath != "" {
		klog.Infof("Using REGISTRY_AUTH_FILE environment variable: %v", registryAuthFilePath)
		return registryAuthFilePath, nil
	}

	// See if the cluster-profile has stored a pull-secret at the conventional location.
	if _, err := os.Stat(ciProfilePullSecretPath); err == nil {
		klog.Infof("Detected %v; using cluster profile for image access", ciProfilePullSecretPath)
		return ciProfilePullSecretPath, nil
	} else if !os.IsNotExist(err) {
		// The file may exist but be unusable, so prefer the next source over
		// selecting a path image extraction would then fail on.
		klog.Warningf("Unable to inspect %v, ignoring it: %v", ciProfilePullSecretPath, err)
	}

	if client == nil {
		klog.Warningf("No cluster client provided; falling back to unauthenticated image access")
		return "", nil
	}

	coreV1, err := client()
	if err != nil {
		return "", fmt.Errorf("unable to create client for the cluster under test: %w", err)
	}
	if coreV1 == nil {
		klog.Warningf("No cluster client available; falling back to unauthenticated image access")
		return "", nil
	}

	// Inspect the cluster-under-test and read its cluster pull-secret dockerconfigjson value.
	clusterPullSecret, err := coreV1.Secrets("openshift-config").Get(ctx, "pull-secret", metav1.GetOptions{})
	if err != nil {
		if kapierrs.IsNotFound(err) {
			klog.Warningf("Cluster has no openshift-config secret/pull-secret; falling back to unauthenticated image access")
			return "", nil
		}
		return "", fmt.Errorf("unable to read ephemeral cluster pull secret: %w", err)
	}

	registryAuthFilePath, err := writeClusterPullSecret(tmpDir, clusterPullSecret.Data[".dockerconfigjson"])
	if err != nil {
		return "", err
	}
	klog.Infof("Using target cluster pull-secrets for registry auth")

	return registryAuthFilePath, nil
}

// writeClusterPullSecret copies the cluster pull secret into tmpDir and returns
// the path it was written to. The file is created exclusively with mode 0600
// rather than written over a predictable name, so a pre-existing file or
// symlink in a shared tmpDir can neither widen the permissions of the copied
// credentials nor redirect the write elsewhere.
func writeClusterPullSecret(tmpDir string, dockerConfigJSON []byte) (string, error) {
	authFile, err := os.CreateTemp(tmpDir, ".dockerconfigjson-")
	if err != nil {
		return "", fmt.Errorf("unable to serialize target cluster pull-secret locally: %w", err)
	}

	_, writeErr := authFile.Write(dockerConfigJSON)
	closeErr := authFile.Close()
	if writeErr != nil || closeErr != nil {
		serializeErr := errors.Join(writeErr, closeErr)
		if removeErr := os.Remove(authFile.Name()); removeErr != nil {
			// The partially written file still holds cluster credentials, so
			// report where it was left behind.
			return "", fmt.Errorf("unable to serialize target cluster pull-secret locally, leaving %q in place: %w", authFile.Name(), errors.Join(serializeErr, removeErr))
		}
		return "", fmt.Errorf("unable to serialize target cluster pull-secret locally: %w", serializeErr)
	}

	return authFile.Name(), nil
}
