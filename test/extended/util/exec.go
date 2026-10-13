package util

import (
	"bytes"
	"fmt"

	v1 "k8s.io/api/core/v1"
	coreclientset "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubernetes/test/e2e/framework"
)

// commandContents fetches the result of invoking a command in the provided container from stdout.
func ExecInPodWithResult(podClient coreclientset.CoreV1Interface, podRESTConfig *rest.Config, ns, name, containerName string, command []string) (string, error) {
	u := podClient.RESTClient().Post().Resource("pods").Namespace(ns).Name(name).SubResource("exec").VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Stdout:    true,
		Stderr:    true,
		Command:   command,
	}, scheme.ParameterCodec).URL()

	e, err := remotecommand.NewSPDYExecutor(podRESTConfig, "POST", u)
	if err != nil {
		return "", fmt.Errorf("could not initialize a new SPDY executor: %v", err)
	}
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	if err := e.Stream(remotecommand.StreamOptions{
		Stdout: buf,
		Stdin:  nil,
		Stderr: errBuf,
	}); err != nil {
		framework.Logf("exec error: %s", errBuf.String())
		return "", err
	}
	return buf.String(), nil
}
