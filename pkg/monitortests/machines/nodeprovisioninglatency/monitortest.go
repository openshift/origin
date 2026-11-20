// Package nodeprovisioninglatency measures how long each Machine takes to get a provider
// instance, a Node object and a Ready Node, and publishes the durations through autodl.
//
// The monitor only collects data and produces no latency pass/fail results. Single-run hard limits
// proved unsustainable for comparable signals such as disruption, so limits should only be
// considered once the autodl data has been analyzed across many runs.
//
// Timestamps come from Machine.metadata.creationTimestamp, the Machine InstanceExists
// condition, Node.metadata.creationTimestamp and the Node Ready condition. The
// InstanceExists timestamp is MAO's first successful observation of the instance, which can
// trail actual cloud creation by up to MAO's requeue interval; InstanceExistsSource records
// whether the condition or the observed phase was used.
//
// Every observed Machine produces a row. Masters that predate the run are usually created by the
// installer, so their timestamps measure install time rather than Machine API provisioning, but a
// Machine API replacement can also predate the run. MachineRole and CreatedDuringRun are exported
// so analysis can separate these cohorts instead of the monitor discarding them.
package nodeprovisioninglatency

import (
	"context"
	"time"

	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type nodeProvisioningLatency struct {
	logger *logrus.Entry

	machineClient machineclient.Interface
	kubeClient    kubernetes.Interface

	state       *collectionState
	unavailable bool

	rows []provisioningRow
}

func NewNodeProvisioningLatency() monitortestframework.MonitorTest {
	return &nodeProvisioningLatency{
		logger: logrus.WithField("monitor", "node-provisioning-latency"),
		state:  newCollectionState(),
	}
}

// markUnavailable logs only a summary of err because client errors embed the request URL, which
// would publish the cluster's internal apiserver address in CI artifacts.
func (m *nodeProvisioningLatency) markUnavailable(reason string, err error) {
	m.unavailable = true
	m.logger.Warnf("%s: %s (node provisioning latency will not be collected)", reason, utility.ErrorSummary(err))
}

func (m *nodeProvisioningLatency) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	machineClient, err := machineclient.NewForConfig(adminRESTConfig)
	if err != nil {
		m.markUnavailable("unable to build Machine client", err)
		return nil
	}

	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		m.markUnavailable("unable to build Kubernetes client", err)
		return nil
	}

	// Clusters without MAPI (HyperShift guests, MicroShift, UPI without Machines) must not
	// produce rows or errors.
	if _, err := machineClient.MachineV1beta1().Machines(machineAPINamespace).List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		m.markUnavailable("unable to list Machines in "+machineAPINamespace, err)
		return nil
	}

	m.machineClient = machineClient
	m.kubeClient = kubeClient

	return nil
}

func (m *nodeProvisioningLatency) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	if m.unavailable || m.machineClient == nil || m.kubeClient == nil {
		return nil
	}

	m.startInformers(ctx)
	return nil
}

func (m *nodeProvisioningLatency) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if m.unavailable || m.machineClient == nil || m.kubeClient == nil {
		return nil, nil, nil
	}

	m.listFinalState(ctx)
	m.rows = m.state.rows(beginning)
	m.logger.Infof("collected %d node provisioning rows", len(m.rows))

	return nil, nil, nil
}

func (m *nodeProvisioningLatency) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals deliberately produces no test cases: this monitor only
// collects data for offline analysis.
func (m *nodeProvisioningLatency) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (m *nodeProvisioningLatency) Cleanup(ctx context.Context) error {
	return nil
}
