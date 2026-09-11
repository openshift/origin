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

func TestBuildTransitionsForCategory(t *testing.T) {
	startTime := time.Date(2022, time.March, 7, 18, 41, 46, 0, time.UTC)
	endTime := startTime.Add(5 * time.Minute)
	containerLocator := monitorapi.NewLocator().ContainerFromNames("namespace", "pod", "uid", "container")
	containerKey := containerLocator.OldLocator()
	locatorKeys := map[string]monitorapi.Locator{containerKey: containerLocator}

	containerStart := monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
		Locator(containerLocator).
		Message(monitorapi.NewMessage().Reason(monitorapi.ContainerReasonContainerStart)).
		Build(startTime.Add(time.Minute), startTime.Add(time.Minute))
	containerWait := monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
		Locator(containerLocator).
		Message(monitorapi.NewMessage().Reason(monitorapi.ContainerReasonContainerWait)).
		Build(startTime.Add(30*time.Second), startTime.Add(30*time.Second))
	containerExit := monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
		Locator(containerLocator).
		Message(monitorapi.NewMessage().Reason(monitorapi.ContainerReasonContainerExit)).
		Build(startTime.Add(2*time.Minute), startTime.Add(2*time.Minute))

	tests := []struct {
		name                    string
		events                  monitorapi.Intervals
		allowMissingInitialWait bool
		wantReasons             []monitorapi.IntervalReason
		wantFirstHumanText      string
	}{
		{
			name:               "standalone topology reports an unobserved initial ContainerWait",
			events:             monitorapi.Intervals{containerStart},
			wantReasons:        []monitorapi.IntervalReason{monitorapi.ContainerReasonContainerWait, monitorapi.ContainerReasonContainerStart},
			wantFirstHumanText: `missed real "ContainerWait"`,
		},
		{
			name:                    "external topology accepts ContainerStart as the first observed lifecycle state",
			events:                  monitorapi.Intervals{containerStart},
			allowMissingInitialWait: true,
			wantReasons:             []monitorapi.IntervalReason{monitorapi.ContainerReasonContainerStart},
		},
		{
			name:                    "external topology preserves a recorded ContainerWait before ContainerStart",
			events:                  monitorapi.Intervals{containerWait, containerStart},
			allowMissingInitialWait: true,
			wantReasons:             []monitorapi.IntervalReason{monitorapi.ContainerReasonContainerWait, monitorapi.ContainerReasonContainerStart},
		},
		{
			name:                    "external topology reports an unobserved ContainerWait before ContainerExit",
			events:                  monitorapi.Intervals{containerExit},
			allowMissingInitialWait: true,
			wantReasons:             []monitorapi.IntervalReason{monitorapi.ContainerReasonContainerWait},
			wantFirstHumanText:      `missed real "ContainerWait"`,
		},
		{
			name:                    "external topology reports an unobserved ContainerWait after a restart",
			events:                  monitorapi.Intervals{containerWait, containerExit, containerStart},
			allowMissingInitialWait: true,
			wantReasons:             []monitorapi.IntervalReason{monitorapi.ContainerReasonContainerWait, monitorapi.ContainerReasonContainerWait, monitorapi.ContainerReasonContainerStart},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := map[string][]monitorapi.Interval{containerKey: tt.events}
			got := buildTransitionsForCategory(
				events,
				locatorKeys,
				monitorapi.ContainerReasonContainerWait,
				monitorapi.ContainerReasonContainerExit,
				newSimpleTimeBounder(startTime, endTime),
				tt.allowMissingInitialWait,
			)

			if assert.Len(t, got, len(tt.wantReasons)) {
				for i, wantReason := range tt.wantReasons {
					assert.Equal(t, wantReason, got[i].Message.Reason)
				}
			}
			if tt.wantFirstHumanText != "" && assert.NotEmpty(t, got) {
				assert.Equal(t, tt.wantFirstHumanText, got[0].Message.HumanMessage)
			}
		})
	}

	t.Run("external topology reports a missing wait after ContainerExit", func(t *testing.T) {
		events := map[string][]monitorapi.Interval{containerKey: {containerWait, containerExit, containerStart}}
		got := buildTransitionsForCategory(
			events,
			locatorKeys,
			monitorapi.ContainerReasonContainerWait,
			monitorapi.ContainerReasonContainerExit,
			newSimpleTimeBounder(startTime, endTime),
			true,
		)

		if assert.Len(t, got, 3) {
			assert.Equal(t, `missed real "ContainerWait"`, got[1].Message.HumanMessage)
		}
	})
}

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
	result := createPodIntervalsFromInstants(inputIntervals, resourceMap, startTime, endTime, false)

	resultBytes, err := monitorserialization.IntervalsToJSON(result)
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
