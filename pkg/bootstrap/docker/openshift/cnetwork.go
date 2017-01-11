package openshift

import (
	"fmt"
)

// TestContainerNetworking launches a container that will check whether the container
// can communicate with the master API and DNS endpoints.
func (h *Helper) TestContainerNetworking(ip string) error {
	// Skip check if the server ip is the localhost
	if ip == "127.0.0.1" {
		return nil
	}
	testCmd := fmt.Sprintf("echo 'Testing connectivity to master API' && "+
		"curl -s -S -k https://%s:8443 && "+
		"echo 'Testing connectivity to master DNS server' && "+
		"for i in {1..10}; do "+
		"   if curl -s -S -k https://kubernetes.default.svc.cluster.local; then "+
		"      exit 0;"+
		"   fi; "+
		"   sleep 1; "+
		"done; "+
		"exit 1", ip)
	_, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		DNS(ip).
		Entrypoint("/bin/bash").
		Command("-c", testCmd).
		Run()
	return err
}
