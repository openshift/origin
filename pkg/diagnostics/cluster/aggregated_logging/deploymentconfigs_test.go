package aggregated_logging

import (
	"errors"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	"github.com/openshift/origin/pkg/diagnostics/log"
)

const (
	testDcPodsKey      = "pods"
	testDcKey          = "deploymentconfigs"
	testSkipAnnotation = "skipAddAnnoation"
)

type fakeDeploymentConfigsDiagnostic struct {
	fakeDiagnostic
	fakePods     kapi.PodList
	fakeDcs      deployapi.DeploymentConfigList
	clienterrors map[string]error
}

func newFakeDeploymentConfigsDiagnostic(t *testing.T) *fakeDeploymentConfigsDiagnostic {
	return &fakeDeploymentConfigsDiagnostic{
		fakeDiagnostic: *newFakeDiagnostic(t),
		clienterrors:   map[string]error{},
	}
}
func (f *fakeDeploymentConfigsDiagnostic) addDeployConfigFor(component string) {
	labels := map[string]string{componentKey: component}
	dc := deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name:   component + "Name",
			Labels: labels,
		},
	}
	f.fakeDcs.Items = append(f.fakeDcs.Items, dc)
}

func (f *fakeDeploymentConfigsDiagnostic) addPodFor(comp string, state kapi.PodPhase) {
	annotations := map[string]string{}
	if comp != testSkipAnnotation {
		annotations[deployapi.DeploymentConfigAnnotation] = comp
	}
	pod := kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:        comp,
			Annotations: annotations,
		},
		Spec: kapi.PodSpec{},
		Status: kapi.PodStatus{
			Phase: state,
		},
	}
	f.fakePods.Items = append(f.fakePods.Items, pod)
}

func (f *fakeDeploymentConfigsDiagnostic) deploymentconfigs(project string, options kapi.ListOptions) (*deployapi.DeploymentConfigList, error) {
	f.test.Logf(">> calling deploymentconfigs: %s", f.clienterrors)
	value, ok := f.clienterrors[testDcKey]
	if ok {
		f.test.Logf(">> error key found..returning %s", value)
		return nil, value
	}
	f.test.Logf(">> error key not found..")
	return &f.fakeDcs, nil
}

func (f *fakeDeploymentConfigsDiagnostic) pods(project string, options kapi.ListOptions) (*kapi.PodList, error) {
	value, ok := f.clienterrors[testDcPodsKey]
	if ok {
		return nil, value
	}
	return &f.fakePods, nil
}

//test client error listing dcs
func TestCheckDcWhenErrorResponseFromClientRetrievingDc(t *testing.T) {
	d := newFakeDeploymentConfigsDiagnostic(t)
	d.clienterrors[testDcKey] = errors.New("error")

	checkDeploymentConfigs(d, d, fakeProject)

	d.assertMessage("AGL0045", "Exp. an error when client returns error retrieving dcs", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckDcWhenNoDeployConfigsFound(t *testing.T) {
	d := newFakeDeploymentConfigsDiagnostic(t)

	checkDeploymentConfigs(d, d, fakeProject)

	d.assertMessage("AGL0047", "Exp. an error when no DeploymentConfigs are found", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckDcWhenOpsOrOtherDeployConfigsMissing(t *testing.T) {
	d := newFakeDeploymentConfigsDiagnostic(t)
	d.addDeployConfigFor(componentNameEs)

	checkDeploymentConfigs(d, d, fakeProject)

	d.assertMessage("AGL0060", "Exp. a warning when ops DeploymentConfigs are missing", log.InfoLevel)
	d.assertMessage("AGL0065", "Exp. an error when non-ops DeploymentConfigs are missing", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckDcWhenClientErrorListingPods(t *testing.T) {
	d := newFakeDeploymentConfigsDiagnostic(t)
	d.clienterrors[testDcPodsKey] = errors.New("New pod error")
	for _, comp := range loggingComponents.List() {
		d.addDeployConfigFor(comp)
	}

	checkDeploymentConfigs(d, d, fakeProject)

	d.assertMessage("AGL0075", "Exp. an error when retrieving pods errors", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckDcWhenNoPodsFoundMatchingDeployConfig(t *testing.T) {
	d := newFakeDeploymentConfigsDiagnostic(t)
	for _, comp := range loggingComponents.List() {
		d.addDeployConfigFor(comp)
	}

	checkDeploymentConfigs(d, d, fakeProject)

	d.assertMessage("AGL0080", "Exp. an error when retrieving pods errors", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckDcWhenInVariousStates(t *testing.T) {
	d := newFakeDeploymentConfigsDiagnostic(t)
	for _, comp := range loggingComponents.List() {
		d.addDeployConfigFor(comp)
		d.addPodFor(comp, kapi.PodRunning)
	}
	d.addPodFor(testSkipAnnotation, kapi.PodRunning)
	d.addPodFor("someothercomponent", kapi.PodPending)
	d.addDeployConfigFor("somerandom component")

	checkDeploymentConfigs(d, d, fakeProject)

	d.assertMessage("AGL0085", "Exp. a warning when pod is missing DeployConfig annotation", log.WarnLevel)
	d.assertMessage("AGL0090", "Exp. an error when pod is not in running state", log.ErrorLevel)
	d.assertMessage("AGL0095", "Exp. an error when pods not found for a DeployConfig", log.ErrorLevel)

	d.dumpMessages()
}
