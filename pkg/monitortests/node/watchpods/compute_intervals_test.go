package watchpods

import (
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestPodIntervalCreation(t *testing.T) {
	files, err := podTests.ReadDir("podTest")
	if err != nil {
		t.Fatal(err)
	}

	podTests := map[string]podIntervalTest{}
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		testName := file.Name()
		events := podBytesOrDie(fmt.Sprintf("podTest/%s/startingEvents.json", testName))
		expected := podStringOrDie(fmt.Sprintf("podTest/%s/expected.json", testName))
		podData := podBytesOrDie(fmt.Sprintf("podTest/%s/podData.json", testName))
		times := podStringOrDie(fmt.Sprintf("podTest/%s/times.txt", testName))
		timeTokens := strings.Split(times, "\n")

		podTest := podIntervalTest{
			events:    events,
			results:   expected,
			startTime: timeTokens[0],
			endTime:   timeTokens[1],
			podData:   [][]byte{podData},
		}
		podTests[testName] = podTest

		t.Logf("%v\n", file.Name())
	}

	for name, test := range podTests {
		t.Run(name, func(t *testing.T) {
			test.test(t)
		})
	}
}

type podIntervalTest struct {
	events    []byte
	results   string
	startTime string
	endTime   string
	podData   [][]byte
}

func (p podIntervalTest) test(t *testing.T) {
	resourceMap := monitorapi.ResourcesMap{
		"pods": monitorapi.InstanceMap{},
	}

	for _, curr := range p.podData {
		if len(curr) == 0 {
			continue
		}

		pod := &corev1.Pod{}
		if err := json.Unmarshal(curr, pod); err != nil {
			t.Fatal(err)
		}
		podMap := resourceMap["pods"]
		instanceKey := monitorapi.InstanceKey{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			UID:       fmt.Sprintf("%v", pod.UID),
		}
		podMap[instanceKey] = pod
		resourceMap["pods"] = podMap
	}

	inputIntervals, err := monitorserialization.EventsFromJSON(p.events)
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
	result := createPodIntervalsFromInstants(inputIntervals, resourceMap, startTime, endTime)

	resultBytes, err := monitorserialization.EventsToJSON(result)
	if err != nil {
		t.Fatal(err)
	}

	resultJSON := string(resultBytes)
	assert.Equal(t, strings.TrimSpace(p.results), resultJSON)
}

//go:embed podTest/*
var podTests embed.FS

func podBytesOrDie(name string) []byte {
	ret, err := podTests.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return ret
}

func podStringOrDie(name string) string {
	ret, err := podTests.ReadFile(name)
	if err != nil {
		panic(err)
	}

	return string(ret)
}
