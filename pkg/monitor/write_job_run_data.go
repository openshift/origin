package monitor

import (
	"context"
	"strings"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// TODO these don't belong here. They should move to a clioptions package about cluster info
func WasMasterNodeUpdated(events monitorapi.Intervals) string {
	nodeUpdates := events.Filter(monitorapi.NodeUpdate)

	for _, i := range nodeUpdates {
		// "locator": "node/ip-10-0-240-32.us-west-1.compute.internal",
		//            "message": "reason/NodeUpdate phase/Update config/rendered-master-757d729d8565a6f9f4e59913d4731db1 roles/control-plane,master reached desired config roles/control-plane,master",
		// vs
		// "locator": "node/ip-10-0-228-209.us-west-1.compute.internal",
		//            "message": "reason/NodeUpdate phase/Update config/rendered-worker-722803a00bad408ee94572ab244ad3bc roles/worker reached desired config roles/worker",
		if strings.Contains(i.Message, "master") {
			return "Y"
		}
	}

	return "N"
}

// TODO these don't belong here. They should move to a clioptions package about cluster info
// TODO this should be taking a client, not a kubeconfig. Can't test a client.
func CollectClusterData(adminKubeConfig *rest.Config, masterNodeUpdated string) platformidentification.ClusterData {
	clusterData := platformidentification.ClusterData{}
	var errs *[]error

	clusterData, errs = platformidentification.BuildClusterData(context.TODO(), adminKubeConfig)

	if errs != nil {
		for _, err := range *errs {
			e2e.Logf("Error building cluster data: %s", err.Error())
		}
		e2e.Logf("Ignoring cluster data due to previous errors: %v", clusterData)
		return platformidentification.ClusterData{}
	}

	clusterData.MasterNodesUpdated = masterNodeUpdated
	return clusterData
}
