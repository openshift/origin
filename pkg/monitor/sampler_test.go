package monitor

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/diff"
)

func TestStartSampling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := NewMonitorWithInterval(5 * time.Millisecond)

	doneCh := make(chan struct{})
	var count int
	m.AddSampler(StartSampling(ctx, m, 21*time.Millisecond, func(previous bool) (condition *Condition, next bool) {
		defer func() { count++ }()
		switch {
		case count <= 5:
			return nil, true
		case count == 6:
			return &Condition{Level: Error, Locator: "tester", Message: "dying"}, false
		case count == 7:
			return &Condition{Level: Info, Locator: "tester", Message: "recovering"}, true
		case count <= 12:
			return nil, true
		case count == 13:
			return &Condition{Level: Error, Locator: "tester", Message: "dying 2"}, false
		case count <= 16:
			return nil, false
		case count == 17:
			return &Condition{Level: Info, Locator: "tester", Message: "recovering 2"}, true
		case count <= 20:
			return nil, true
		default:
			doneCh <- struct{}{}
			return nil, true
		}
	}).ConditionWhenFailing(&Condition{
		Level:   Error,
		Locator: "tester",
		Message: "down",
	}))

	m.StartSampling(ctx)
	<-doneCh
	cancel()

	var describe []string
	var log []string
	events := m.Events(time.Time{}, time.Time{})
	for _, interval := range events {
		i := interval.To.Sub(interval.From)
		describe = append(describe, fmt.Sprintf("%v %s", *interval.Condition, i))
		log = append(log, fmt.Sprintf("%v", *interval.Condition))
	}

	zero := time.Time{}.String()
	expected := []string{
		fmt.Sprintf("{2 tester dying %s}", zero),
		fmt.Sprintf("{2 tester down %s}", zero),
		fmt.Sprintf("{0 tester recovering %s}", zero),
		fmt.Sprintf("{2 tester dying 2 %s}", zero),
		fmt.Sprintf("{2 tester down %s}", zero),
		fmt.Sprintf("{0 tester recovering 2 %s}", zero),
	}
	if !reflect.DeepEqual(log, expected) {
		t.Fatalf("%s", diff.ObjectReflectDiff(log, expected))
	}
	if events[4].To.Sub(events[4].From) < 2*events[1].To.Sub(events[1].From) {
		t.Fatalf("last condition should be at least 2x first condition length:\n%s", strings.Join(describe, "\n"))
	} else {
		t.Logf("%s", strings.Join(describe, "\n"))
	}
}
