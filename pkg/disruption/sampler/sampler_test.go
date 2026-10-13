package sampler

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSampler(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	fake := &fakeProducerConsumer{
		t: t,
		r: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	runner := NewWithProducerConsumer(10*time.Millisecond, fake)
	stopped := runner.Run(ctx)

	<-ctx.Done()
	t.Logf("sampler context timed out")
	<-stopped.Done()
	t.Logf("sampler is done")
	if err := fake.verify(); err != nil {
		t.Errorf("%v", err)
	}
}

type fakeProducerConsumer struct {
	t       *testing.T
	samples []*Sample
	count   int64 // atomic
	closed  bool
	lock    sync.Mutex
	r       *rand.Rand
}

func (f *fakeProducerConsumer) Produce(stop context.Context, id uint64) (interface{}, error) {
	count := atomic.AddInt64(&f.count, 1)
	// the first few sample(s) should complete after stop,
	// to ensure that they are not dropped by the collector.
	if count <= 5 {
		<-stop.Done()
	}

	f.lock.Lock()
	r := f.r.Float64()
	f.lock.Unlock()

	time.Sleep(time.Duration(float64(time.Second) * r))
	return nil, nil
}

func (f *fakeProducerConsumer) Consume(s *Sample, _ interface{}) {
	f.samples = append(f.samples, s)
	f.t.Logf("%s", s)
}

func (f *fakeProducerConsumer) Close() {
	f.closed = true
}

func (f *fakeProducerConsumer) verify() error {
	count := atomic.LoadInt64(&f.count)
	// If the sampler generated N sample(s), we must consume N sample(s)
	if count == 0 || count != int64(len(f.samples)) {
		return fmt.Errorf("expected %d samples collected, but got: %d", count, int64(len(f.samples)))
	}
	// The resulting samples must arrive in 1, 2, 3 ... N sequence
	if !sort.IsSorted(SortedByID(f.samples)) {
		return fmt.Errorf("expected resulting samples to be in 1, 2, 3, ... N sequence")
	}
	if !sort.IsSorted(SortedByStartedAt(f.samples)) {
		return fmt.Errorf("expected resulting samples to be sorted by StartedAt")
	}
	if !f.closed {
		return fmt.Errorf("expected Close to be invoked")
	}
	return nil
}
