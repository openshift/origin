package upgrade

import (
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"
)

func clusterOperatorsForRendering(ctx context.Context, c configv1client.Interface) string {
	clusterOperators, err := c.ConfigV1().ClusterOperators().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err.Error()
	}

	retBytes, err := json.Marshal(clusterOperators)
	if err != nil {
		return err.Error()
	}

	return string(retBytes)
}
