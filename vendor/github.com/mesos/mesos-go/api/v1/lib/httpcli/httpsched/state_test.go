package httpsched

import (
	"errors"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib/encoding"
	"github.com/mesos/mesos-go/api/v1/lib/extras/latch"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

func TestDisconnectionDecoder(t *testing.T) {

	// invoke disconnect upon decoder errors
	expected := errors.New("unmarshaler error")
	decoder := encoding.DecoderFunc(func(_ encoding.Unmarshaler) error { return expected })
	latch := new(latch.L).Reset()

	d := disconnectionDecoder(decoder, latch.Close)
	err := d.Decode(nil)
	if err != expected {
		t.Errorf("expected %v instead of %v", expected, err)
	}
	if !latch.Closed() {
		t.Error("disconnect func was not called")
	}

	// ERROR event triggers disconnect
	latch.Reset()
	errtype := scheduler.Event_ERROR
	event := &scheduler.Event{Type: errtype}
	decoder = encoding.DecoderFunc(func(um encoding.Unmarshaler) error { return nil })
	d = disconnectionDecoder(decoder, latch.Close)
	_ = d.Decode(event)
	if !latch.Closed() {
		t.Error("disconnect func was not called")
	}

	// sanity: non-ERROR event does not trigger disconnect
	latch.Reset()
	errtype = scheduler.Event_SUBSCRIBED
	event = &scheduler.Event{Type: errtype}
	_ = d.Decode(event)
	if latch.Closed() {
		t.Error("disconnect func was unexpectedly called")
	}

	// non scheduler.Event objects trigger disconnect
	latch.Reset()
	nonEvent := &scheduler.Call{}
	_ = d.Decode(nonEvent)
	if !latch.Closed() {
		t.Error("disconnect func was not called")
	}
}
