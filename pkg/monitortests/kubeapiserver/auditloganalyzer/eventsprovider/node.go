package eventsprovider

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func NewNodeEvents(client kubernetes.Interface, nodeName string, beginning, end *v1.MicroTime) *NodeEvents {
	return &NodeEvents{
		APIServerEvents: NewAPIServerEvents(nodeName, "kube-apiserver", beginning, end),
	}
}

type NodeEvents struct {
	*APIServerEvents
}
