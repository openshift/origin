package util

import (
	"encoding/base64"
	"fmt"
	"strings"

	utilimage "github.com/openshift/origin/test/extended/util/image"
)

// CopyImageToInternalRegistry copies source to dest using the namespace's builder service account.
func CopyImageToInternalRegistry(oc *CLI, namespace, source, dest string) (string, error) {
	if oc.Namespace() != namespace {
		return "", fmt.Errorf("CLI namespace %q does not match destination namespace %q", oc.Namespace(), namespace)
	}

	executor, err := NewPodExecutor(oc, "registry-image-copy", utilimage.ShellImage())
	if err != nil {
		return "", err
	}

	token, err := serviceAccountToken(oc, "builder", namespace)
	if err != nil {
		return "", err
	}
	destination := strings.TrimPrefix(dest, "docker://")
	registry, _, found := strings.Cut(destination, "/")
	if !found {
		return "", fmt.Errorf("destination image %q does not include a repository", dest)
	}
	config := fmt.Sprintf(`{"auths":{%q:{"auth":%q}}}`, registry, base64.StdEncoding.EncodeToString([]byte("dnm:"+token)))
	configEncoded := base64.StdEncoding.EncodeToString([]byte(config))
	if _, err := executor.Exec("echo " + configEncoded + " | base64 -d > /tmp/registry-config.json"); err != nil {
		return "", err
	}

	return executor.Exec(fmt.Sprintf("oc image mirror %s %s --insecure=true --registry-config /tmp/registry-config.json", strings.TrimPrefix(source, "docker://"), destination))
}

func serviceAccountToken(oc *CLI, serviceAccount, namespace string) (string, error) {
	token, err := oc.AsAdmin().WithoutNamespace().Run("create").Args("token", serviceAccount, "-n", namespace).Output()
	if err == nil {
		return token, nil
	}
	if !strings.Contains(token, "unknown command") {
		return "", err
	}
	return oc.AsAdmin().WithoutNamespace().Run("sa").Args("get-token", serviceAccount, "-n", namespace).Output()
}
