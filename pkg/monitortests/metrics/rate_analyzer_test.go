package metrics

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	prometheustypes "github.com/prometheus/common/model"
)

func TestRateSeriesAnalyzer(t *testing.T) {
	tests := []struct {
		name             string
		query            fakeQuery
		errShouldContain string
		disruptions      []interval
		count            int
	}{
		{
			name:             "query returns error",
			query:            "{}",
			errShouldContain: "query returned error, monitor: fake-monitor, err: ",
		},
		{
			name:             "query returns wrong type",
			query:            "wrong type", // special case to return an unexpected "string" type
			errShouldContain: "expected a prometheus Matrix type, but got: \"string\", monitor: fake-monitor",
		},
		{
			name: "series has no disruption",
			query: `
[
  {
    "metric": {
      "host": "api-int.ostest.test.metalkube.org:6443"
    },
    "values": [
      [
        1723587909,
        "0"
      ],
      [
        1723587969,
        "0"
      ],
      [
        1723588029,
        "0"
      ]
    ]
  },
  {
    "metric": {
      "host": "172.30.0.1:443"
    },
    "values": [
      [
        1723587909,
        "0"
      ]
    ]
  }
]`,
			errShouldContain: "",
			count:            2,
		},
		{
			name: "single value series with no disruption",
			query: `
[
  {
    "metric": {
      "host": "172.30.0.1:443"
    },
    "values": [
      [
        1723587909,
        "0"
      ]
    ]
  }
]`,
			errShouldContain: "",
			count:            1,
		},
		{
			name: "single value series with a disruption",
			query: `
[
  {
    "metric": {
      "host": "172.30.0.1:443"
    },
    "values": [
      [
        1723587909,
        "0.1"
      ]
    ]
  }
]`,
			errShouldContain: "",
			count:            1,
			disruptions: []interval{
				{
					From: prometheustypes.SamplePair{
						Timestamp: prometheustypes.TimeFromUnix(1723587909),
						Value:     prometheustypes.SampleValue(0.1),
					},
					To: prometheustypes.SamplePair{
						Timestamp: prometheustypes.TimeFromUnix(1723587909),
						Value:     prometheustypes.SampleValue(0.1),
					},
				},
			},
		},
		{
			name: "series with multiple  disruptions",
			query: `
[
  {
 "metric": {
      "host": "api-int.ostest.test.metalkube.org:6443"
    },
    "values": [
      [
        1723589469,
        "0"
      ],
      [
        1723589529,
        "0.3"
      ],
      [
        1723589589,
        "0.1"
      ],
      [
        1723589649,
        "0.2"
      ],
      [
        1723589709,
        "0"
      ],
      [
        1723589769,
        "0.1"
      ],
      [
        1723589829,
        "0.2"
      ]
    ]
  }
]`,
			errShouldContain: "",
			count:            1,
			disruptions: []interval{
				{
					From: prometheustypes.SamplePair{
						Timestamp: prometheustypes.TimeFromUnix(1723589529),
						Value:     prometheustypes.SampleValue(0.3),
					},
					To: prometheustypes.SamplePair{
						Timestamp: prometheustypes.TimeFromUnix(1723589649),
						Value:     prometheustypes.SampleValue(0.2),
					},
				},
				{
					From: prometheustypes.SamplePair{
						Timestamp: prometheustypes.TimeFromUnix(1723589769),
						Value:     prometheustypes.SampleValue(0.1),
					},
					To: prometheustypes.SamplePair{
						Timestamp: prometheustypes.TimeFromUnix(1723589829),
						Value:     prometheustypes.SampleValue(0.2),
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			callback := &fakeCallback{}

			analyzer := RateSeriesAnalyzer{}
			err := analyzer.Analyze(context.Background(), test.query, time.Time{}, time.Time{}, callback)

			switch {
			case len(test.errShouldContain) > 0:
				if err == nil || !strings.Contains(err.Error(), test.errShouldContain) {
					t.Errorf("expected error to contain %q, but got %v", test.errShouldContain, err)
				}
			default:
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if want, got := test.disruptions, callback.disruptions; !reflect.DeepEqual(want, got) {
				t.Errorf("unexpected disruptions: %s", cmp.Diff(want, got))
			}
			if want, got := test.count, callback.countStart; want != got {
				t.Errorf("expected series start count: %d, but got: %d", want, got)
			}
			if want, got := test.count, callback.countEnd; want != got {
				t.Errorf("expected series start count: %d, but got: %d", want, got)
			}
		})
	}
}

type fakeQuery string

func (q fakeQuery) RunQuery(ctx context.Context, start, end time.Time) (prometheustypes.Value, error) {
	// special case to return a wrong type
	if q == "wrong type" {
		return &prometheustypes.String{}, nil
	}

	var matrix prometheustypes.Matrix
	if err := json.Unmarshal([]byte(q), &matrix); err != nil {
		return nil, err
	}
	return matrix, nil
}

type interval struct {
	From, To prometheustypes.SamplePair
}

type fakeCallback struct {
	countStart, countEnd int
	disruptions          []interval
}

func (c *fakeCallback) Name() string { return "fake-monitor" }

func (c *fakeCallback) StartSeries(metric prometheustypes.Metric) { c.countStart++ }

func (c *fakeCallback) EndSeries() { c.countEnd++ }

func (c *fakeCallback) NewInterval(metric prometheustypes.Metric, from, to *prometheustypes.SamplePair) {
	c.disruptions = append(c.disruptions, interval{From: *from, To: *to})
}
