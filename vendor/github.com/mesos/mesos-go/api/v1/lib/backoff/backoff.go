package backoff

import (
	"fmt"
	"reflect"
	"time"
)

func BurstNotifier(burst int, minWait, maxWait time.Duration, until <-chan struct{}) <-chan struct{} {
	if burst < 1 {
		return nil // no limit
	}
	if burst == 1 {
		return Notifier(minWait, maxWait, until)
	}

	// build a synamic select/case statement based on burst size
	cases := make([]reflect.SelectCase, burst+1)
	for i := 0; i < burst; i++ {
		ch := Notifier(minWait, maxWait, until)
		cases[i].Dir = reflect.SelectRecv
		cases[i].Chan = reflect.ValueOf(ch)
	}
	cases[burst].Dir = reflect.SelectRecv
	cases[burst].Chan = reflect.ValueOf(until)

	// listen for tokens emitted by child buckets and forward them to the tokens chan
	tokens := make(chan struct{})
	go func() {
		defer close(tokens)
		for {
			i, _, _ := reflect.Select(cases)
			if i == burst {
				// special case: this is the "until" chan
				return
			}
			// otherwise we got a signal from a child bucket that we need to forward
			select {
			case tokens <- struct{}{}:
			case <-until:
				return
			}
		}
	}()
	return tokens
}

// Notifier returns a chan that yields a struct{}{} every so often. the wait period
// between structs is between minWait and maxWait. greedy consumers that continuously read
// from the returned chan will see the wait period generally increase.
//
// Note: this func panics if minWait is a non-positive value to avoid busy-looping.
func Notifier(minWait, maxWait time.Duration, until <-chan struct{}) <-chan struct{} {
	// TODO(jdef) add jitter to this func
	if maxWait < minWait {
		maxWait, minWait = minWait, maxWait
	}
	if minWait <= 0 {
		panic(fmt.Sprintf("illegal value for minWait: %v", minWait))
	}
	tokens := make(chan struct{})
	limiter := tokens
	go func() {
		d := 0 * time.Second
		t := time.NewTimer(d)
		defer t.Stop()
		for {
			select {
			case limiter <- struct{}{}:
				d *= 2
				if d > maxWait {
					d = maxWait
				}
				limiter = nil
				// drain the timer to avoid Reset problems
				if !t.Stop() {
					<-t.C
				}
			case <-t.C:
				if limiter != nil {
					d /= 2
				} else {
					limiter = tokens
				}
			case <-until:
				return
			}
			// important to have non-zero minWait otherwise we busy-loop
			if d < minWait {
				d = minWait
			}
			t.Reset(d)
		}
	}()
	return tokens
}
