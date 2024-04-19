package nodestateanalyzer

import (
	"embed"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntervalsFromEvents_NodeChanges(t *testing.T) {
	// regenerate this file if schema changes by getting an updated e2e-events file, finding a worker node, and running:
	// cat e2e-events_20240308-085144.json | jq '.items |= map(select(.locator.keys.node == "ci-op-0774jw7y-f9945-k9278-worker-a-c2l9q") | select(.message.reason? | match("RegisteredNode|Starting|Cordon|Drain|NodeNotSchedulable|OSUpdateStarted|InClusterUpgrade|OSUpdateStaged|PendingConfig|Reboot|DiskPressure|MemoryPressure|PIDPressure|Ready|NodeNotReady|NotReady")))' > pkg/monitortests/node/nodestateanalyzer/testdata/node.json
	intervals, err := monitorserialization.EventsFromFile("testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	changes := intervalsFromEvents_NodeChanges(intervals, nil, time.Time{}, time.Now())
	for _, c := range changes {
		t.Logf("%s - %s", c.From.UTC().Format(time.RFC3339), c.Message.OldMessage())
		assert.Equal(t, "node-lifecycle-constructor", c.Message.Annotations[monitorapi.AnnotationConstructed])
	}

	require.Equal(t, 4, len(changes))
	assert.Equal(t, "NodeUpdate", string(changes[0].Message.Reason))
	assert.Equal(t, "Drain", changes[0].Message.Annotations[monitorapi.AnnotationPhase])
	assert.Contains(t, "drained node", changes[0].Message.HumanMessage)

	assert.Equal(t, "NodeUpdate", string(changes[1].Message.Reason))
	assert.Equal(t, "OperatingSystemUpdate", changes[1].Message.Annotations[monitorapi.AnnotationPhase])
	assert.Contains(t, "updated operating system", changes[1].Message.HumanMessage)

	assert.Equal(t, "NodeUpdate", string(changes[2].Message.Reason))
	assert.Equal(t, "Reboot", changes[2].Message.Annotations[monitorapi.AnnotationPhase])
	assert.Contains(t, "rebooted and kubelet started", changes[2].Message.HumanMessage)

	assert.Equal(t, "NotReady", string(changes[3].Message.Reason))
	assert.Contains(t, "node is not ready", changes[3].Message.HumanMessage)
}

func TestNodeUpdateCreation(t *testing.T) {
	files, err := nodeTests.ReadDir("nodeTest")
	if err != nil {
		t.Fatal(err)
	}

	nodeTests := map[string]nodeIntervalTest{}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		testName := file.Name()
		events := nodeBytesOrDie(fmt.Sprintf("nodeTest/%s/startingEvents.json", testName))
		expected := nodeStringOrDie(fmt.Sprintf("nodeTest/%s/expected.json", testName))
		times := nodeStringOrDie(fmt.Sprintf("nodeTest/%s/times.txt", testName))
		timeTokens := strings.Split(times, "\n")

		nodeTest := nodeIntervalTest{
			events:    events,
			results:   expected,
			startTime: timeTokens[0],
			endTime:   timeTokens[1],
		}
		nodeTests[testName] = nodeTest

		t.Logf("%v\n", file.Name())
	}

	for name, test := range nodeTests {
		t.Run(name, func(t *testing.T) {
			test.test(t)
		})
	}
}

type nodeIntervalTest struct {
	events    []byte
	results   string
	startTime string
	endTime   string
}

func (p nodeIntervalTest) test(t *testing.T) {
	inputIntervals, err := monitorserialization.IntervalsFromJSON(p.events)
	if err != nil {
		t.Fatal(err)
	}
	startTime, err := time.Parse(time.RFC3339, p.startTime)
	if err != nil {
		t.Fatal(err)
	}
	endTime, err := time.Parse(time.RFC3339, p.endTime)
	if err != nil {
		t.Fatal(err)
	}
	result := intervalsFromEvents_NodeChanges(inputIntervals, nil, startTime, endTime)

	resultBytes, err := monitorserialization.IntervalsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	resultJSON := string(resultBytes)
	assert.Equal(t, strings.TrimSpace(p.results), resultJSON)
}

//go:embed nodeTest/*
var nodeTests embed.FS

func nodeBytesOrDie(name string) []byte {
	ret, err := nodeTests.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return ret
}

func nodeStringOrDie(name string) string {
	ret, err := nodeTests.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return string(ret)
}
