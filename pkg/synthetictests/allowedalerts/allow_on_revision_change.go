package allowedalerts

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/synthetictests/historicaldata"

	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/origin/pkg/synthetictests/platformidentification"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type etcdRevisionChangeAllowance struct {
	operatorClient   operatorv1client.OperatorV1Interface
	startingRevision int
}

var allowedWhenEtcdRevisionChange = &etcdRevisionChangeAllowance{}

func NewAllowedWhenEtcdRevisionChange(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (*etcdRevisionChangeAllowance, error) {
	etcd, err := operatorClient.Etcds().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	biggestRevision := 0
	for _, nodeStatus := range etcd.Status.NodeStatuses {
		if int(nodeStatus.CurrentRevision) > biggestRevision {
			biggestRevision = int(nodeStatus.CurrentRevision)
		}
	}
	return &etcdRevisionChangeAllowance{
		operatorClient:   operatorClient,
		startingRevision: biggestRevision,
	}, nil
}

func (d *etcdRevisionChangeAllowance) FailAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	etcd, err := d.operatorClient.Etcds().Get(context.TODO(), "cluster", metav1.GetOptions{})
	if err != nil {
		return historicaldata.DurationOrDie(6.022)
	}
	biggestRevision := 0
	for _, nodeStatus := range etcd.Status.NodeStatuses {
		if int(nodeStatus.CurrentRevision) > biggestRevision {
			biggestRevision = int(nodeStatus.CurrentRevision)
		}
	}
	numberOfRevisions := biggestRevision - d.startingRevision

	if numberOfRevisions > 2 {
		return time.Duration(numberOfRevisions) * 10 * time.Minute

	}

	allowed, _, _ := getClosestPercentilesValues(alertName, jobType)
	return allowed.P99
}

func (d *etcdRevisionChangeAllowance) FlakeAfter(alertName string, jobType platformidentification.JobType) time.Duration {
	allowed, _, _ := getClosestPercentilesValues(alertName, jobType)
	return allowed.P95
}
