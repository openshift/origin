package origin

import (
	"sync"
	"testing"
)

func innerWaitingMethod(counter, finished *sync.WaitGroup, stopCh <-chan struct{}) {
	counter.Done()
	<-stopCh
	finished.Done()
}

func outerWaitingMethod(innerCounter, innerFinishedCounter, outerCounter *sync.WaitGroup, innerStopCh, outerStopCh <-chan struct{}) {
	go innerWaitingMethod(innerCounter, innerFinishedCounter, innerStopCh)

	outerCounter.Done()
	<-outerStopCh
}

func TestStuckMethodCount(t *testing.T) {
	innerCounter := &sync.WaitGroup{}
	outerCounter := &sync.WaitGroup{}
	innerFinishedCounter := &sync.WaitGroup{}
	innerStop := make(chan struct{})
	outerStop := make(chan struct{})
	for i := 0; i < 5; i++ {
		innerCounter.Add(1)
		innerFinishedCounter.Add(1)
		outerCounter.Add(1)
		go outerWaitingMethod(innerCounter, innerFinishedCounter, outerCounter, innerStop, outerStop)
	}

	innerChecker := &TooManyThreadsStuckHealth{
		name:           "test",
		targetFunction: "github.com/openshift/origin/pkg/cmd/server/origin.innerWaitingMethod",
		limit:          3,
	}
	outerChecker := &TooManyThreadsStuckHealth{
		name:           "test",
		targetFunction: "github.com/openshift/origin/pkg/cmd/server/origin.outerWaitingMethod",
		limit:          3,
	}

	outerCounter.Wait()
	innerCounter.Wait()

	if count := innerChecker.Count(); count != 5 {
		t.Errorf("expected 5, got %v", count)
	}
	if count := outerChecker.Count(); count != 5 {
		t.Errorf("expected 5, got %v", count)
	}

	close(innerStop)
	innerFinishedCounter.Wait()

	if count := innerChecker.Count(); count != 0 {
		t.Errorf("expected 0, got %v", count)
	}
	if count := outerChecker.Count(); count != 5 {
		t.Errorf("expected 5, got %v", count)
	}

	close(outerStop)
}
