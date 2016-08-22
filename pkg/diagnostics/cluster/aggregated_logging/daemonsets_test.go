package aggregated_logging

import (
	"errors"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapisext "k8s.io/kubernetes/pkg/apis/extensions"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

const (
	testPodsKey  = "pods"
	testNodesKey = "nodes"
	testDsKey    = "daemonsets"
)

type fakeDaemonSetDiagnostic struct {
	fakeDiagnostic
	fakePods       kapi.PodList
	fakeNodes      kapi.NodeList
	fakeDaemonsets kapisext.DaemonSetList
	clienterrors   map[string]error
}

func newFakeDaemonSetDiagnostic(t *testing.T) *fakeDaemonSetDiagnostic {
	return &fakeDaemonSetDiagnostic{
		fakeDiagnostic: *newFakeDiagnostic(t),
		clienterrors:   map[string]error{},
	}
}

func (f *fakeDaemonSetDiagnostic) addDsPodWithPhase(state kapi.PodPhase) {
	pod := kapi.Pod{
		Spec: kapi.PodSpec{},
		Status: kapi.PodStatus{
			Phase: state,
		},
	}
	f.fakePods.Items = append(f.fakePods.Items, pod)
}

func (f *fakeDaemonSetDiagnostic) addDaemonSetWithSelector(key string, value string) {
	selector := map[string]string{key: value}
	ds := kapisext.DaemonSet{
		Spec: kapisext.DaemonSetSpec{
			Template: kapi.PodTemplateSpec{
				Spec: kapi.PodSpec{
					NodeSelector: selector,
				},
			},
			Selector: &unversioned.LabelSelector{MatchLabels: selector},
		},
	}
	f.fakeDaemonsets.Items = append(f.fakeDaemonsets.Items, ds)
}

func (f *fakeDaemonSetDiagnostic) addNodeWithLabel(key string, value string) {
	labels := map[string]string{key: value}
	node := kapi.Node{
		ObjectMeta: kapi.ObjectMeta{
			Labels: labels,
		},
	}
	f.fakeNodes.Items = append(f.fakeNodes.Items, node)
}

func (f *fakeDaemonSetDiagnostic) daemonsets(project string, options kapi.ListOptions) (*kapisext.DaemonSetList, error) {
	value, ok := f.clienterrors[testDsKey]
	if ok {
		return nil, value
	}
	return &f.fakeDaemonsets, nil
}

func (f *fakeDaemonSetDiagnostic) nodes(options kapi.ListOptions) (*kapi.NodeList, error) {
	value, ok := f.clienterrors[testNodesKey]
	if ok {
		return nil, value
	}
	return &f.fakeNodes, nil
}

func (f *fakeDaemonSetDiagnostic) pods(project string, options kapi.ListOptions) (*kapi.PodList, error) {
	value, ok := f.clienterrors[testPodsKey]
	if ok {
		return nil, value
	}
	return &f.fakePods, nil
}

func TestCheckDaemonsetsWhenErrorResponseFromClientRetrievingDaemonsets(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.clienterrors[testDsKey] = errors.New("someerror")

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0405", "Exp. error when client errors on retrieving DaemonSets", log.ErrorLevel)
}

func TestCheckDaemonsetsWhenNoDaemonsetsFound(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0407", "Exp. error when client retrieves no DaemonSets", log.ErrorLevel)
}

func TestCheckDaemonsetsWhenErrorResponseFromClientRetrievingNodes(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.clienterrors[testNodesKey] = errors.New("someerror")
	d.addDaemonSetWithSelector("foo", "bar")

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0410", "Exp. error when client errors on retrieving Nodes", log.ErrorLevel)
}

func TestCheckDaemonsetsWhenDaemonsetsMatchNoNodes(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.addDaemonSetWithSelector("foo", "bar")
	d.addNodeWithLabel("foo", "xyz")

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0420", "Exp. error when daemonsets do not match any nodes", log.ErrorLevel)
}

func TestCheckDaemonsetsWhenDaemonsetsMatchPartialNodes(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.addDaemonSetWithSelector("foo", "bar")
	d.addNodeWithLabel("foo", "bar")
	d.addNodeWithLabel("foo", "xyz")

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0425", "Exp. warning when daemonsets matches less then all the nodes", log.WarnLevel)
}

func TestCheckDaemonsetsWhenClientErrorsFetchingPods(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.clienterrors[testPodsKey] = errors.New("some error")
	d.addDaemonSetWithSelector("foo", "bar")
	d.addNodeWithLabel("foo", "bar")

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0438", "Exp. error when there is an error retrieving pods for a daemonset", log.ErrorLevel)

	d.dumpMessages()
}

func TestCheckDaemonsetsWhenNoPodsMatchDaemonSet(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.addDaemonSetWithSelector("foo", "bar")
	d.addNodeWithLabel("foo", "bar")

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0440", "Exp. error when there are no pods that match a daemonset", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckDaemonsetsWhenNoPodsInRunningState(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.addDaemonSetWithSelector("foo", "bar")
	d.addNodeWithLabel("foo", "bar")
	d.addDsPodWithPhase(kapi.PodPending)

	checkDaemonSets(d, d, fakeProject)

	d.assertMessage("AGL0445", "Exp. error when there are no pods in running state", log.ErrorLevel)
	d.dumpMessages()
}

func TestCheckDaemonsetsWhenAllPodsInRunningState(t *testing.T) {
	d := newFakeDaemonSetDiagnostic(t)
	d.addDaemonSetWithSelector("foo", "bar")
	d.addNodeWithLabel("foo", "bar")
	d.addDsPodWithPhase(kapi.PodRunning)

	checkDaemonSets(d, d, fakeProject)

	d.assertNoErrors()
	d.dumpMessages()
}
