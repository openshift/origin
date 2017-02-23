package openshift

const podNetworkTestCmd = `#!/bin/bash
set -e
echo 'Testing connectivity to master API'
for i in {1..40}; do 
	if curl -s -S -k -m 1 https://172.30.0.1:8443; then 
	  continue
	fi
done
if [[ ! $? -eq 0 ]]; then
    exit 1
fi
echo 'Testing connectivity to master DNS server'
for i in {1..40}; do
   if curl -s -S -k -m 1 https://kubernetes.default.svc.cluster.local; then
      exit 0
   fi
   sleep 1
done
exit 1
`

// TestContainerNetworking launches a container that will check whether the container
// can communicate with the master API and DNS endpoints.
func (h *Helper) TestContainerNetworking() error {
	_, err := h.runHelper.New().Image(h.image).
		DiscardContainer().
		DNS("172.30.0.1").
		Entrypoint("/bin/bash").
		Command("-c", podNetworkTestCmd).
		Run()
	return err
}
