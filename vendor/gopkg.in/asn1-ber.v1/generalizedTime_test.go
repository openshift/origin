package ber

import (
	"testing"
	"time"
)

func TestParseGeneralizedTime(t *testing.T) {
	for _, test := range []struct {
		str    string
		wanted time.Time
		err    error
	}{
		{
			"20170222190527Z",
			time.Date(2017, time.Month(2), 22, 19, 05, 27, 0, time.UTC),
			nil,
		},
		{
			"201702221905Z",
			time.Date(2017, time.Month(2), 22, 19, 05, 0, 0, time.UTC),
			nil,
		},
		{
			"2017022219Z",
			time.Date(2017, time.Month(2), 22, 19, 0, 0, 0, time.UTC),
			nil,
		},
		{
			"2017022219.25Z",
			time.Date(2017, time.Month(2), 22, 19, 15, 0, 0, time.UTC),
			nil,
		},
		{
			"201702221905.25Z",
			time.Date(2017, time.Month(2), 22, 19, 5, 15, 0, time.UTC),
			nil,
		},
		{
			"20170222190525-0100",
			time.Date(2017, time.Month(2), 22, 19, 5, 25, 0, time.FixedZone("", -3600)),
			nil,
		},
		{
			"20170222190525+0100",
			time.Date(2017, time.Month(2), 22, 19, 5, 25, 0, time.FixedZone("", 3600)),
			nil,
		},
		{
			"20170222190525+01",
			time.Date(2017, time.Month(2), 22, 19, 5, 25, 0, time.FixedZone("", 3600)),
			nil,
		},
		{
			"20170222190527.123Z",
			time.Date(2017, time.Month(2), 22, 19, 05, 27, 123*1000*1000, time.UTC),
			nil,
		},
		{
			"20170222190527,123Z",
			time.Date(2017, time.Month(2), 22, 19, 05, 27, 123*1000*1000, time.UTC),
			nil,
		},
		{
			"2017022219-0100",
			time.Date(2017, time.Month(2), 22, 19, 0, 0, 0, time.FixedZone("", -3600)),
			nil,
		},
	} {
		genTime, err := ParseGeneralizedTime([]byte(test.str))
		if err != nil {
			if test.err != nil {
				if err != test.err {
					t.Errorf("unexpected error in %s: %s", test.str, err)
				}
			} else {
				t.Errorf("test %s failed with error: %s", test.str, err)
			}
		} else {
			if !genTime.Equal(test.wanted) {
				t.Errorf("test got unexpected result: wanted=%s, got=%s", test.wanted, genTime)
			}
		}
	}
}
