package allowedalerts

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestlibrary/historicaldata"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type etcdRevisionChangeAllowance struct {
	numberOfRevisionDuringTest int
}

var allowedWhenEtcdRevisionChange = &etcdRevisionChangeAllowance{}
var _ AlertTestAllowanceCalculator = &etcdRevisionChangeAllowance{}

func NewAllowedWhenEtcdRevisionChange(ctx context.Context, kubeClient kubernetes.Interface, duration time.Duration) (*etcdRevisionChangeAllowance, error) {
	numberOfRevisions, err := GetEstimatedNumberOfRevisionsForEtcdOperator(ctx, kubeClient, duration)
	if err != nil {
		return nil, err
	}
	return &etcdRevisionChangeAllowance{
		numberOfRevisionDuringTest: numberOfRevisions,
	}, nil
}

func (d *etcdRevisionChangeAllowance) FailAfter(key historicaldata.AlertDataKey) (time.Duration, error) {
	// if the number of revisions is different compared to what we have collected at the beginning of the test suite
	// increase allowed time for the alert
	// the rationale is that some tests might roll out a new version of etcd during each rollout we allow max 3 elections per revision (we assume there are 3 master machines at most)
	// in the future, we could make this function more dynamic
	// we will leave it simple for now
	if d.numberOfRevisionDuringTest > 2 {
		return time.Duration(d.numberOfRevisionDuringTest) * 15 * time.Minute, nil

	}
	allowed, _, _ := getClosestPercentilesValues(key)
	return allowed.P99, nil
}

func (d *etcdRevisionChangeAllowance) FlakeAfter(key historicaldata.AlertDataKey) time.Duration {
	allowed, _, _ := getClosestPercentilesValues(key)
	return allowed.P95
}

// GetEstimatedNumberOfRevisionsForEtcdOperator calculates the number of revisions that have occurred between now and duration
func GetEstimatedNumberOfRevisionsForEtcdOperator(ctx context.Context, kubeClient kubernetes.Interface, duration time.Duration) (int, error) {
	configMaps, err := kubeClient.CoreV1().ConfigMaps("openshift-etcd").List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	var revisionCounter int
	allowedRevisionsTS := time.Now().Add(-duration)
	for _, configMap := range configMaps.Items {
		if strings.Contains(configMap.Name, "revision-status-") {
			sub := strings.Split(configMap.Name, "-")
			if len(sub) != 3 {
				return 0, fmt.Errorf("the configmap: %v has an incorrect name, unable to extract the revision number", configMap.Name)
			}
			_, err := strconv.Atoi(sub[2])
			if err != nil {
				return 0, err
			}
			if configMap.CreationTimestamp.After(allowedRevisionsTS) {
				revisionCounter++
			}
		}
	}
	return revisionCounter, nil
}
