package maxconnections

import (
	"context"
	"testing"
	"time"
)

func TestLimiter(t *testing.T) {
	const timeout = 1 * time.Second

	maxRunning := 2
	maxInQueue := 3
	maxWaitInQueue := time.Duration(1) // any non-zero value, we redefine newTimer for this test.
	lim := NewLimiter(maxRunning, maxInQueue, maxWaitInQueue)

	// All clients in the queue will be rejected when the channel deadline is closed.
	deadline := make(chan time.Time)
	lim.(*limiter).newTimer = func(d time.Duration) *time.Timer {
		t := time.NewTimer(d)
		t.C = deadline
		return t
	}

	ctx := context.Background()
	c := newCounter()
	jobBarrier := make(chan struct{}, maxRunning+maxInQueue+1)
	done := make(chan struct{})
	wait := func(reason string) {
		select {
		case <-done:
		case <-time.After(timeout):
			t.Fatal(reason)
		}
	}
	for i := 0; i < maxRunning+maxInQueue+1; i++ {
		go func() {
			started := lim.Start(ctx)
			defer func() {
				c.Add(started, 1)
				done <- struct{}{}
			}()
			if started {
				<-jobBarrier
				lim.Done()
			}
		}()
	}

	wait("timeout while waiting one failed job")

	// expected state: 2 running, 3 in queue, 1 failed
	if expected := (countM{false: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	jobBarrier <- struct{}{}
	wait("timeout while waiting one succeed job")

	// expected state: 2 running, 2 in queue, 1 failed, 1 succeed
	if expected := (countM{false: 1, true: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	close(deadline)
	wait("timeout while waiting the first failed job from the queue")
	wait("timeout while waiting the second failed job from the queue")

	// expected state: 2 running, 0 in queue, 3 failed, 1 succeed
	if expected := (countM{false: 3, true: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	jobBarrier <- struct{}{}
	jobBarrier <- struct{}{}
	wait("timeout while waiting the first succeed job")
	wait("timeout while waiting the second succeed job")

	// expected state: 0 running, 0 in queue, 3 failed, 3 succeed
	if expected := (countM{false: 3, true: 3}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}
}

func TestLimiterContext(t *testing.T) {
	const timeout = 1 * time.Second

	maxRunning := 2
	maxInQueue := 3
	maxWaitInQueue := 120 * time.Second
	lim := NewLimiter(maxRunning, maxInQueue, maxWaitInQueue)

	type job struct {
		ctx      context.Context
		cancel   context.CancelFunc
		finished bool
	}
	c := newCounter()
	jobs := make(chan *job, maxRunning+maxInQueue+1)
	jobBarrier := make(chan struct{}, maxRunning+maxInQueue+1)
	done := make(chan struct{})
	startJobs := func(amount int) {
		for i := 0; i < amount; i++ {
			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				job := &job{
					ctx:      ctx,
					cancel:   cancel,
					finished: false,
				}
				jobs <- job

				started := lim.Start(ctx)
				defer func() {
					c.Add(started, 1)
					job.finished = true
					done <- struct{}{}
				}()
				if started {
					// if the job is running, it is not cancellable anymore
					<-jobBarrier
					lim.Done()
				}
			}()
		}
	}
	cancelJobs := func(amount int, desc string) {
		i := 0
		for i < amount {
			select {
			case job := <-jobs:
				if job.finished {
					continue
				}
				job.cancel()
				i++
			case <-time.After(timeout):
				t.Fatalf("timeout while cancelling %s (%d of %d)", desc, i+1, amount)
			}
		}
	}
	finishJobs := func(amount int, desc string) {
		for i := 0; i < amount; i++ {
			select {
			case jobBarrier <- struct{}{}:
			case <-time.After(timeout):
				t.Fatalf("timeout while finishing %s (%d of %d)", desc, i+1, amount)
			}
		}
	}
	waitJobs := func(amount int, desc string) {
		for i := 0; i < amount; i++ {
			select {
			case <-done:
			case <-time.After(timeout):
				t.Fatalf("timeout while waiting %s (%d of %d)", desc, i+1, amount)
			}
		}
	}

	startJobs(maxRunning + maxInQueue + 1)
	waitJobs(1, "the job that doesn't fit in the queue from the first portion")

	// expected state: 2 running, 3 in queue, 1 failed
	if expected := (countM{false: 1}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	cancelJobs(maxRunning+maxInQueue, "the jobs from the first portion")
	// The running jobs is not cancellable in this test.
	waitJobs(maxInQueue, "the cancelled jobs from the queue from the first portion")

	// expected state: 2 running, 0 in queue, 4 failed
	if expected := (countM{false: 4}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	startJobs(maxInQueue + 1)
	waitJobs(1, "the job that doesn't fit in the queue from the second portion")

	// expected state: 2 running, 3 in queue, 5 failed
	if expected := (countM{false: 5}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}

	finishJobs(maxRunning+maxInQueue, "all running and queued jobs")
	waitJobs(maxRunning+maxInQueue, "all finished jobs")

	// expected state: 0 running, 0 in queue, 5 failed, 5 succeed
	if expected := (countM{false: 5, true: 5}); !c.Equal(expected) {
		t.Errorf("c = %v, want %v", c.Values(), expected)
	}
}
