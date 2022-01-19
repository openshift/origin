package clusterstatus

import (
	"context"
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
	openshiftcorev1 "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/rest"
)

const infraResourceName = "cluster"

func GetClusterInfraStatus(ctx context.Context, restClient *rest.Config) (*configv1.InfrastructureStatus, error) {
	client, err := openshiftcorev1.NewForConfig(restClient)
	if err != nil {
		return nil, err
	}
	infra, err := client.Infrastructures().Get(ctx, infraResourceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if infra == nil {
		return nil, fmt.Errorf("getting resource Infrastructure (name: %s) succeeded but object was nil", infraResourceName)
	}
	return &infra.Status, nil
}
