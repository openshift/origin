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
	StartTime    time.Time     `json:"startTime"`
	TestDuration time.Duration `json:"testDuration"`
}

func (td TestDuration) printLog() error {
	b, err := json.Marshal(td)
	fmt.Println(string(b))
	return err
}

func (td TestDuration) MarshalJSON() ([]byte, error) {
	type Alias TestDuration
	return json.Marshal(&struct {
		Alias
		TestDuration string `json:"testDuration"`
	}{
		Alias:        (Alias)(td),
		TestDuration: td.TestDuration.String(),
	})
}

func (td *TestDuration) UnmarshalJSON(b []byte) error {
	var err error
	type Alias TestDuration
	s := &struct {
		TestDuration string `json:"testDuration"`
		*Alias
	}{
		Alias: (*Alias)(td),
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	td.TestDuration, err = time.ParseDuration(s.TestDuration)
	if err != nil {
		return err
	}
	return nil
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

func NewTestDuration(name string, startTime time.Time, testDuration time.Duration) TestDuration {
	return TestDuration{
		BaseMetrics: BaseMetrics{
			Marker: marker_name,
			Name:   name,
			Type:   fmt.Sprintf("%T", (*TestDuration)(nil))[1:]},
		StartTime:    startTime,
		TestDuration: testDuration,
	}
}
