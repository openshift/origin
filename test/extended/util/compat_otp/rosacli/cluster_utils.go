package rosacli

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	ClusterDescriptionComputeDesired    = "Compute (desired)"
	ClusterDescriptionComputeAutoscaled = "Compute (autoscaled)"
)

func RetrieveDesiredComputeNodes(clusterDescription *ClusterDescription) (nodesNb int, err error) {
	if clusterDescription.Nodes[0]["Compute (desired)"] != nil {
		var isInt bool
		nodesNb, isInt = clusterDescription.Nodes[0]["Compute (desired)"].(int)
		if !isInt {
			err = fmt.Errorf("'%v' is not an integer value", clusterDescription.Nodes[0]["Compute (desired)"])
		}
	} else {
		// Try autoscale one
		autoscaleInfo := clusterDescription.Nodes[0]["Compute (Autoscaled)"].(string)
		nodesNb, err = strconv.Atoi(strings.Split(autoscaleInfo, "-")[0])
	}
	return
}
