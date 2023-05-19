package sampler

import (
	"context"
	"sync"
	"time"
)

// Runner will run the sampler asynchronously, it returns a context
// immediately that the caller can wait on.
// the done context is cancelled when:
//   - all the goroutines have completed
//   - all samples have been collected
type Runner interface {
	Run(stop context.Context) (done context.Context)
}

// Producer is responsible for producing a single sample
// the function should return an error if it fails,
// custom can be used to send domain specific data to the collector.
type Producer interface {
	Produce(ctx context.Context, sampleID uint64) (interface{}, error)
}

type ProducerFunc func(context.Context, uint64) (customData interface{}, err error)

func (f ProducerFunc) Produce(ctx context.Context, sampleID uint64) (interface{}, error) {
	return f(ctx, sampleID)
}

// Consumer receives a resulting Sample
type Consumer interface {
	Consume(s *Sample, custom interface{})
	Close()
}

type ProducerConsumer interface {
	Producer
	Consumer
}

func NewWithProducerConsumer(interval time.Duration, pc ProducerConsumer) Runner {
	return &sampler{interval: interval, producer: pc, consumer: pc}
}

type result struct {
	// custom holds the custom data returned by the Producer
	custom interface{}
	sample *Sample
}

type sampler struct {
	interval time.Duration
	producer Producer
	consumer Consumer
}

func (s sampler) Run(stop context.Context) context.Context {
	resultCh, producerDoneCh := produce(stop, s.interval, s.producer)
	consumerDoneCh := consume(resultCh, s.consumer)

	done, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		<-producerDoneCh
		<-consumerDoneCh
	}()
	return done
}

func produce(stop context.Context, interval time.Duration, p Producer) (<-chan result, <-chan struct{}) {
	resultCh := make(chan result, 1)
	producerDoneCh := make(chan struct{})
	go func() {
		wg := sync.WaitGroup{}
		ticker := time.NewTicker(interval)
		defer func() {
			// wait for all the sample generating goroutines to be done.
			wg.Wait()

			ticker.Stop()
			// closing resultCh ensures that the consumer reading
			// from this channel will terminate.
			close(resultCh)
			close(producerDoneCh)
		}()

		waitCh := make(chan struct{})
		// the first goroutine never waits to write to the resultCh channel
		close(waitCh)
		sequence := uint64(0)
		for {
			wg.Add(1)
			sequence += 1
			now := time.Now()

			thisOneDoneCh := make(chan struct{})
			go func(id uint64, at time.Time, waitCh <-chan struct{}, doneCh chan struct{}) {
				defer wg.Done()
				defer close(doneCh)

				sample := &Sample{ID: id, StartedAt: at}
				result := result{sample: sample}
				func() {
					defer func() {
						sample.FinishedAt = time.Now()
					}()
					result.custom, sample.Err = p.Produce(stop, sample.ID)
				}()

				// we want the write to the resultCh channel be in order as well
				// this guarantees that the consumer will see the samples
				// in order of generation: 1, 2, 3 ... n
				select {
				case <-waitCh:
					resultCh <- result
				}
			}(sequence, now, waitCh, thisOneDoneCh)

			// the next goroutine will wait for this channel to be closed
			waitCh = thisOneDoneCh

			select {
			case <-ticker.C:
			case <-stop.Done():
				return
			}
		}
	}()

	return resultCh, producerDoneCh
}

func consume(resultCh <-chan result, consumer Consumer) <-chan struct{} {
	consumerDoneCh := make(chan struct{})
	go func() {
		defer close(consumerDoneCh)
		for {
			select {
			case pair, ok := <-resultCh:
				if !ok {
					// no more values in the channel
					consumer.Close()
					return
				}
				consumer.Consume(pair.sample, pair.custom)
			}
		}
	}()
	return consumerDoneCh
}
