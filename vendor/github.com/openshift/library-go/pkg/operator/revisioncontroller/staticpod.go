package revisioncontroller

import (
	"fmt"

	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// StaticPodLatestRevisionClient is an LatestRevisionClient implementation for StaticPodOperatorStatus.
type StaticPodLatestRevisionClient struct {
	v1helpers.StaticPodOperatorClient
}

var _ LatestRevisionClient = StaticPodLatestRevisionClient{}

func (c StaticPodLatestRevisionClient) GetLatestRevisionState() (*operatorv1.OperatorSpec, *operatorv1.OperatorStatus, int32, string, error) {
	spec, status, rv, err := c.GetStaticPodOperatorState()
	if err != nil {
		return nil, nil, 0, "", err
	}
	return &spec.OperatorSpec, &status.OperatorStatus, status.LatestAvailableRevision, rv, nil
}

func (c StaticPodLatestRevisionClient) UpdateLatestRevisionOperatorStatus(latestAvailableRevision int32, updateFuncs ...v1helpers.UpdateStatusFunc) (*operatorv1.OperatorStatus, bool, error) {
	staticPodUpdateFuncs := make([]v1helpers.UpdateStaticPodStatusFunc, 0, len(updateFuncs))
	for _, f := range updateFuncs {
		staticPodUpdateFuncs = append(staticPodUpdateFuncs, func(operatorStatus *operatorv1.StaticPodOperatorStatus) error {
			return f(&operatorStatus.OperatorStatus)
		})
	}
	status, changed, err := v1helpers.UpdateStaticPodStatus(c, append(staticPodUpdateFuncs, func(status *operatorv1.StaticPodOperatorStatus) error {
		if status.LatestAvailableRevision == latestAvailableRevision {
			klog.Warningf("revision %d is unexpectedly already the latest available revision. This is a possible race!", latestAvailableRevision)
			return fmt.Errorf("conflicting latestAvailableRevision %d", status.LatestAvailableRevision)
		}
		status.LatestAvailableRevision = latestAvailableRevision
		return nil
	})...)
	if err != nil {
		return nil, false, err
	}
	return &status.OperatorStatus, changed, nil
}
