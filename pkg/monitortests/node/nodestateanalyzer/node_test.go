package nodestateanalyzer

import (
	"embed"
	"fmt"
	"strings"
	"testing"
	"time"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/stretchr/testify/assert"
)

func TestIntervalsFromEvents_NodeChanges(t *testing.T) {
	intervals, err := monitorserialization.EventsFromFile("testdata/node.json")
	if err != nil {
		t.Fatal(err)
	}
	changes := intervalsFromEvents_NodeChanges(intervals, nil, time.Time{}, time.Now())
	for _, c := range changes {
		t.Logf("%s - %s", c.From.UTC().Format(time.RFC3339), c.Message)
	}
	//out, _ := monitorserialization.EventsIntervalsToJSON(changes)
	assert.Equal(t, 3, len(changes))
	assert.Equal(t, "constructed/node-lifecycle-constructor phase/Drain reason/NodeUpdate roles/worker drained node",
		changes[0].Message, "unexpected event")
	assert.Equal(t, "constructed/node-lifecycle-constructor phase/OperatingSystemUpdate reason/NodeUpdate roles/worker updated operating system",
		changes[1].Message, "unexpected event")
	assert.Equal(t, "constructed/node-lifecycle-constructor phase/Reboot reason/NodeUpdate roles/worker rebooted and kubelet started",
		changes[2].Message, "unexpected event")
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
