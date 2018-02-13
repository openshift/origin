package openshift

import (
	"fmt"
)

const podNetworkTestCmd = `#!/bin/bash
set -e
echo 'Testing connectivity to master API'
failed=0
for i in {1..40}; do
	if curl -s -S -k -m 1 https://${MASTER_IP}:8443; then
	  failed=0
	  break
	else
	  failed=1
	fi
done
if [[ failed -eq 1 ]]; then
    exit 1
fi
echo 'Testing connectivity to master DNS server'
for i in {1..40}; do
   if curl -s -S -k -m 1 https://kubernetes.default.svc.cluster.local; then
      exit 0
   fi
done
exit 1
`

// TestContainerNetworking launches a container that will check whether the container
// can communicate with the master API and DNS endpoints.
func (h *Helper) TestContainerNetworking(ip string) error {
	_, _, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		Env(fmt.Sprintf("MASTER_IP=%s", ip)).
		DNS("172.30.0.1").
		Entrypoint("/bin/bash").
		Command("-c", podNetworkTestCmd).
		Run()
	return err
}
