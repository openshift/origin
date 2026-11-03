// Package nodeprovisioninglatency measures how long each Machine takes to get a provider
// instance, a Node object and a Ready Node, and publishes the durations through autodl so
// regressions can be detected across runs.
//
// Timestamps come from Machine.metadata.creationTimestamp, the Machine InstanceExists
// condition, Node.metadata.creationTimestamp and the Node Ready condition. The
// InstanceExists timestamp is MAO's first successful observation of the instance, which can
// trail actual cloud creation by up to MAO's requeue interval; InstanceExistsSource records
// whether the condition or the observed phase was used.
package nodeprovisioninglatency

import (
	"context"
	"time"

	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type nodeProvisioningLatency struct {
	flakeJunits monitortestframework.FlakeJunits
	logger      *logrus.Entry

	machineClient   machineclient.Interface
	kubeClient      kubernetes.Interface
	adminRESTConfig *rest.Config

	state       *collectionState
	unavailable bool

	beginning time.Time
	rows      []provisioningRow
}

func NewNodeProvisioningLatency(flakeJunits monitortestframework.FlakeJunits) monitortestframework.MonitorTest {
	return &nodeProvisioningLatency{
		flakeJunits: flakeJunits,
		logger:      logrus.WithField("monitor", "node-provisioning-latency"),
		state:       newCollectionState(),
	}
}

func (m *nodeProvisioningLatency) markUnavailable(format string, args ...interface{}) {
	m.unavailable = true
	m.logger.Warnf(format+" (node provisioning latency will not be collected)", args...)
}

func (m *nodeProvisioningLatency) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	m.adminRESTConfig = adminRESTConfig

	machineClient, err := machineclient.NewForConfig(adminRESTConfig)
	if err != nil {
		m.markUnavailable("unable to build Machine client: %v", err)
		return nil
	}

	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		m.markUnavailable("unable to build Kubernetes client: %v", err)
		return nil
	}

	// Clusters without MAPI (HyperShift guests, MicroShift, UPI without Machines) must not
	// produce rows, junits or errors.
	if _, err := machineClient.MachineV1beta1().Machines(machineAPINamespace).List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		m.markUnavailable("unable to list Machines in %s: %v", machineAPINamespace, err)
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
	if m.unavailable {
		return nil, nil, nil
	}

	m.beginning = beginning
	m.listFinalState(ctx)
	m.rows = m.state.rows(beginning)
	m.logger.Infof("collected %d node provisioning rows", len(m.rows))

	return nil, nil, nil
}

func (m *nodeProvisioningLatency) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (m *nodeProvisioningLatency) Cleanup(ctx context.Context) error {
	return nil
}
