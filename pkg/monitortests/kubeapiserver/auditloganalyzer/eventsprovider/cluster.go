package eventsprovider

import (
	"context"
	"errors"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/kubernetes"
)

type ClusterEvents struct {
	beginning *time.Time
	end       *time.Time
	errs      []error
}

func NewClusterEvents(beginning, end *time.Time) *ClusterEvents {
	return &ClusterEvents{
		beginning: beginning,
		end:       end,
	}
}

func (p *ClusterEvents) Err() error {
	return errors.Join(p.errs...)
}

func (p *ClusterEvents) Events(ctx context.Context, client kubernetes.Interface) <-chan *auditv1.Event {
	out := make(chan *auditv1.Event)
	masterOnly, err := labels.NewRequirement("node-role.kubernetes.io/master", selection.Exists, nil)
	if err != nil {
		p.errs = []error{err}
		close(out)
		return out
	}
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*masterOnly).String(),
	})
	if err != nil {
		p.errs = []error{err}
		close(out)
		return out
	}

	var microBeginning, microEnd *metav1.MicroTime
	if nil != p.beginning {
		micro := metav1.NewMicroTime(*p.beginning)
		microBeginning = &micro
	}
	if nil != p.end {
		micro := metav1.NewMicroTime(*p.end)
		microEnd = &micro
	}

	var wg sync.WaitGroup
	p.errs = make([]error, len(nodes.Items))
	for i, node := range nodes.Items {
		wg.Add(1)
		go func(nodeName string) {
			defer wg.Done()
			np := NewNodeEvents(client, nodeName, microBeginning, microEnd)
			for v := range np.Events(ctx, client) {
				out <- v
			}
			p.errs[i] = np.Err()
		}(node.Name)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
