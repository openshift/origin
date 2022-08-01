package nodedetails

import (
	"context"
	"io/ioutil"

	"k8s.io/client-go/kubernetes"
)

// GetNodeLog returns logs for a particular systemd service on a given node.
// We're count on these logs to fit into some reasonable memory size.
func GetNodeLog(ctx context.Context, client kubernetes.Interface, nodeName, systemdServiceName string) ([]byte, error) {
	path := client.CoreV1().RESTClient().Get().
		Namespace("").Name(nodeName).
		Resource("nodes").SubResource("proxy", "logs").Suffix("journal").URL().Path

	req := client.CoreV1().RESTClient().Get().RequestURI(path).
		SetHeader("Accept", "text/plain, */*")
	req.Param("since", "-1d")
	req.Param("unit", systemdServiceName)

	in, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer in.Close()

	return ioutil.ReadAll(in)
}
