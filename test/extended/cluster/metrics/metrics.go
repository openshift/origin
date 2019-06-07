package metrics

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	marker_name string = "cluster_loader_marker"
)

type Metrics interface {
	printLog() error
}

type BaseMetrics struct {
	// To let the 3rd party know that this log entry is important
	// TODO set this up by config file
	Marker string `json:"marker"`
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type TestDuration struct {
	BaseMetrics
	StartTime    time.Time      `json:"startTime"`
	TestDuration time.Duration  `json:"testDuration"`
	Steps        []StepDuration `json:"Steps"`
}

type StepDuration interface {
}

type BaseStepDuration struct {
	Type      string        `json:"type"`
	StartTime time.Time     `json:"startTime"`
	TotalTime time.Duration `json:"totalTime"`
}

type TemplateStepDuration struct {
	BaseStepDuration
	RateDelay      time.Duration `json:"rateDelay"`
	RateDelayCount int           `json:"rateDelayCount"`
	StepPause      time.Duration `json:"stepPause"`
	StepPauseCount int           `json:"stepPauseCount"`
	SyncTime       time.Duration `json:"syncTime"`
}

type PodStepDuration struct {
	BaseStepDuration
	WaitPodsDurations []time.Duration `json:"waitPodsDurations"`
	RateDelay         time.Duration   `json:"rateDelay"`
	RateDelayCount    int             `json:"rateDelayCount"`
	StepPause         time.Duration   `json:"stepPause"`
	StepPauseCount    int             `json:"stepPauseCount"`
	SyncTime          time.Duration   `json:"syncTime"`
}

func (td TestDuration) printLog() error {
	b, err := json.Marshal(td)
	fmt.Println(string(b))
	return err
}

func LogMetrics(metrics []Metrics) error {
	for _, m := range metrics {
		err := m.printLog()
		if err != nil {
			return err
		}
	}
	return nil
}

func NewTestDuration(name string, startTime time.Time, testDuration time.Duration, steps []StepDuration) TestDuration {
	return TestDuration{
		BaseMetrics: BaseMetrics{
			Marker: marker_name,
			Name:   name,
			Type:   fmt.Sprintf("%T", (*TestDuration)(nil))[1:]},
		StartTime:    startTime,
		TestDuration: testDuration,
		Steps:        steps,
	}
}

func NewTemplateStepDuration(rateDelay, stepPause time.Duration) TemplateStepDuration {
	return TemplateStepDuration{
		BaseStepDuration: BaseStepDuration{StartTime: time.Now(),
			Type: fmt.Sprintf("%T", (*TemplateStepDuration)(nil))[1:]},
		RateDelay:      rateDelay,
		RateDelayCount: 0,
		StepPause:      stepPause,
		StepPauseCount: 0,
	}
}

func NewPodStepDuration(rateDelay, stepPause time.Duration) PodStepDuration {
	return PodStepDuration{
		BaseStepDuration: BaseStepDuration{StartTime: time.Now(),
			Type: fmt.Sprintf("%T", (*PodStepDuration)(nil))[1:]},
		RateDelay:         rateDelay,
		RateDelayCount:    0,
		StepPause:         stepPause,
		StepPauseCount:    0,
		WaitPodsDurations: []time.Duration{},
	}
}
