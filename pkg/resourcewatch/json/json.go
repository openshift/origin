package json

import (
	"context"
	"encoding/json"
	"io"

	"github.com/go-logr/logr"

	"github.com/openshift/origin/pkg/resourcewatch/observe"
)

func Source(file io.ReadCloser) (observe.ObservationSource, error) {
	decoder := json.NewDecoder(file)

	return func(ctx context.Context, log logr.Logger, resourceC chan<- *observe.ResourceObservation) chan struct{} {
		finished := make(chan struct{})
		go func() {
			defer func() {
				file.Close()
				close(finished)
			}()

			for decoder.More() {
				// Exit if the context is cancelled.
				if ctx.Err() != nil {
					return
				}

				observation := &observe.ResourceObservation{}
				if err := decoder.Decode(observation); err != nil {
					log.Error(err, "Failed to decode observation", err)
					return
				}
				resourceC <- observation
			}
		}()
		return finished
	}, nil
}

func Sink(file io.WriteCloser) (observe.ObservationSink, error) {
	encoder := json.NewEncoder(file)

	return func(log logr.Logger, resourceC <-chan *observe.ResourceObservation) chan struct{} {
		finished := make(chan struct{})
		go func() {
			defer func() {
				file.Close()
				close(finished)
			}()

			for observation := range resourceC {
				if err := encoder.Encode(observation); err != nil {
					log.Error(err, "Failed to encode observation", err)
					return
				}
			}
		}()
		return finished
	}, nil
}
