package util

import (
	"fmt"
	"strings"

	utilimage "github.com/openshift/origin/test/extended/util/image"
)

const skopeoImage = "quay.io/openshifttest/skopeo@sha256:d5f288968744a8880f983e49870c0bfcf808703fe126e4fb5fc393fb9e599f65"

// CopyImageToInternalRegistry copies source to dest using the namespace's builder service account.
func CopyImageToInternalRegistry(oc *CLI, namespace, source, dest string) (string, error) {
	const appName = "skopeo"

	podName, err := findPodByLabel(oc, namespace, "name="+appName)
	if err != nil {
		return "", err
	}
	if podName == "" {
		if err := oc.Run("run").Args(appName, "--image="+utilimage.LocationFor(skopeoImage), "--restart=Never", "--labels=name="+appName, "--command", "--", "bash", "-c", "while :; do sleep 15m; done", "-n", namespace).Execute(); err != nil {
			return "", err
		}
		podName, err = findPodByLabel(oc, namespace, "name="+appName)
		if err != nil {
			return "", err
		}
		if podName == "" {
			return "", fmt.Errorf("Skopeo pod was not created in namespace %q", namespace)
		}
		if err := AssertPodToBeReady(oc, podName, namespace); err != nil {
			return "", err
		}
	} else if err := AssertPodToBeReady(oc, podName, namespace); err != nil {
		return "", err
	}

	token, err := serviceAccountToken(oc, "builder", namespace)
	if err != nil {
		return "", err
	}

	return oc.AsAdmin().WithoutNamespace().Run("exec").Args(
		podName, "-n", namespace, "--", appName,
		"--insecure-policy", "--src-tls-verify=false", "--dest-tls-verify=false",
		"copy", "--dcreds", "dnm:"+token, source, dest,
	).Output()
}

func findPodByLabel(oc *CLI, namespace, label string) (string, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", "-n", namespace, "-l", label, "-o", `jsonpath={.items[*].metadata.name}`).Output()
	if err != nil {
		return "", err
	}
	pods := strings.Fields(output)
	if len(pods) == 0 {
		return "", nil
	}
	return pods[0], nil
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
