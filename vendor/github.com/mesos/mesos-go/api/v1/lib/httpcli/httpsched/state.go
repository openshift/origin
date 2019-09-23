package httpsched

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/encoding"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli/apierrors"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
)

const (
	headerMesosStreamID = "Mesos-Stream-Id"
	debug               = false
)

type StateError string

func (err StateError) Error() string { return string(err) }

var (
	errMissingStreamID   = httpcli.ProtocolError("missing Mesos-Stream-Id header expected with successful SUBSCRIBE")
	errAlreadySubscribed = StateError("already subscribed, cannot re-issue a SUBSCRIBE call")
)

type (
	// state implements calls.Caller and tracks connectivity with Mesos
	state struct {
		client *client // client is a handle to the original underlying HTTP client

		m      sync.Mutex
		fn     stateFn      // fn is the next state function to execute
		caller calls.Caller // caller is (maybe) used by a state function to execute a call

		call *scheduler.Call // call is the next call to execute
		resp mesos.Response  // resp is the Mesos response from the most recently executed call
		err  error           // err is the error from the most recently executed call
	}

	stateFn func(context.Context, *state) (stateFn, Notification)
)

func maybeLogged(f httpcli.DoFunc) httpcli.DoFunc {
	if debug {
		return func(req *http.Request) (*http.Response, error) {
			log.Println("wrapping request", req.URL, req.Header)
			resp, err := f(req)
			if err == nil {
				log.Printf("status %d", resp.StatusCode)
				for k := range resp.Header {
					log.Println("header " + k + ": " + resp.Header.Get(k))
				}
			}
			return resp, err
		}
	}
	return f
}

// DisconnectionDetector is a programmable response decorator that attempts to detect errors
// that should transition the state from "connected" to "disconnected". Detector implementations
// are expected to invoke the `disconnect` callback in order to initiate the disconnection.
//
// The default implementation will transition to a disconnected state when:
//   - an error occurs while decoding an object from the subscription stream
//   - mesos reports an ERROR-type scheduler.Event object via the subscription stream
//   - an object on the stream does not decode to a *scheduler.Event (sanity check)
//
// Consumers of this package may choose to override default behavior by overwriting the default
// value of this var, but should exercise caution: failure to properly transition to a disconnected
// state may cause subsequent Call operations to fail (without recourse).
var DisconnectionDetector = func(disconnect func()) mesos.ResponseDecorator {
	var disconnectOnce sync.Once
	disconnectF := func() { disconnectOnce.Do(disconnect) }
	closeF := mesos.CloseFunc(func() (_ error) { disconnectF(); return })
	return mesos.ResponseDecoratorFunc(func(resp mesos.Response) mesos.Response {
		return &mesos.ResponseWrapper{
			Response: resp,
			Decoder:  disconnectionDecoder(resp, disconnectF),
			Closer:   closeF,
		}
	})
}

func disconnectionDecoder(decoder encoding.Decoder, disconnect func()) encoding.Decoder {
	return encoding.DecoderFunc(func(u encoding.Unmarshaler) (err error) {
		err = decoder.Decode(u)
		if err != nil {
			disconnect()
			return
		}
		switch e := u.(type) {
		case (*scheduler.Event):
			if e.GetType() == scheduler.Event_ERROR {
				// the mesos scheduler API recommends that scheduler implementations
				// resubscribe in this case. we initiate the disconnection here because
				// it is assumed to be convenient for most framework implementations.
				disconnect()
			}
		default:
			// sanity check: this should never happen in practice.
			err = httpcli.ProtocolError(
				fmt.Sprintf("unexpected object on subscription event stream: %v", e))
			disconnect()
		}
		return
	})
}

func disconnectedFn(ctx context.Context, state *state) (stateFn, Notification) {
	// (a) validate call = SUBSCRIBE
	if state.call.GetType() != scheduler.Call_SUBSCRIBE {
		state.resp = nil
		state.err = apierrors.CodeUnsubscribed.Error("")
		return disconnectedFn, withoutNotification
	}

	// (b) prepare client for a subscription call
	var (
		mesosStreamID = ""
		undoable      = httpcli.WrapDoer(func(f httpcli.DoFunc) httpcli.DoFunc {
			f = maybeLogged(f)
			return func(req *http.Request) (resp *http.Response, err error) {
				resp, err = f(req)
				if err == nil && resp.StatusCode == 200 {
					// grab Mesos-Stream-Id header; if missing then
					// close the response body and return an error
					mesosStreamID = resp.Header.Get(headerMesosStreamID)
					if mesosStreamID == "" {
						resp.Body.Close()
						resp = nil
						err = errMissingStreamID
					}
				}
				return
			}
		})
		subscribeCaller = &callerTemporary{
			opt:            undoable,
			callerInternal: state.client,
			requestOpts:    []httpcli.RequestOpt{httpcli.Close(true)},
		}
	)

	// (c) execute the call, save the result in resp, err
	stateResp, stateErr := subscribeCaller.Call(ctx, state.call)
	state.err = stateErr

	// (d) if err != nil return disconnectedFn since we're unsubscribed
	if stateErr != nil {
		if stateResp != nil {
			stateResp.Close()
		}
		state.resp = nil
		return disconnectedFn, withoutNotification
	}

	transitionToDisconnected := func() {
		defer stateResp.Close() // swallow any error here

		state.m.Lock()
		defer state.m.Unlock()
		state.fn = disconnectedFn
		state.client.notify(Notification{Type: NotificationDisconnected})
	}

	// wrap the response: any errors processing the subscription stream should result in a
	// transition to a disconnected state ASAP.
	state.resp = DisconnectionDetector(transitionToDisconnected).Decorate(stateResp)

	// (e) else prepare callerTemporary w/ special header, return connectedFn since we're now subscribed
	state.caller = &callerTemporary{
		opt:            httpcli.DefaultHeader(headerMesosStreamID, mesosStreamID),
		callerInternal: state.client,
	}
	return connectedFn, Notification{Type: NotificationConnected}
}

func errorIndicatesSubscriptionLoss(err error) (result bool) {
	type lossy interface {
		SubscriptionLoss() bool
	}
	if lossyErr, ok := err.(lossy); ok {
		result = lossyErr.SubscriptionLoss()
	}
	return
}

func connectedFn(ctx context.Context, state *state) (stateFn, Notification) {
	// (a) validate call != SUBSCRIBE
	if state.call.GetType() == scheduler.Call_SUBSCRIBE {
		if state.client.allowReconnect {
			// Reset internal state back to DISCONNECTED and re-execute the SUBSCRIBE call.
			// Mesos will hangup on the old SUBSCRIBE socket after this one completes.
			state.caller = nil
			state.resp = nil
			state.err = nil
			state.fn = disconnectedFn
			state.client.notify(Notification{Type: NotificationDisconnected})

			return state.fn(ctx, state)
		} else {
			state.resp = nil

			// TODO(jdef) not super happy with this error: I don't think that mesos minds if we issue
			// redundant subscribe calls. However, the state tracking mechanism in this module can't
			// cope with it (e.g. we'll need to track a new stream-id, etc).
			// We make a best effort to transition to a disconnected state if we detect protocol errors,
			// error events, or mesos-generated "not subscribed" errors. But we don't handle things such
			// as, for example, authentication errors. Granted, the initial subscribe call should fail
			// if authentication is an issue, so we should never end up here. I'm not convinced there's
			// not other edge cases though with respect to other error codes.
			state.err = errAlreadySubscribed
			return connectedFn, withoutNotification
		}
	}

	// (b) execute call, save the result in resp, err
	state.resp, state.err = state.caller.Call(ctx, state.call)

	if errorIndicatesSubscriptionLoss(state.err) {
		// properly transition back to a disconnected state if mesos thinks that we're unsubscribed
		return disconnectedFn, Notification{Type: NotificationDisconnected}
	}

	// stay connected, don't attempt to interpret other errors here
	return connectedFn, withoutNotification
}

func (state *state) Call(ctx context.Context, call *scheduler.Call) (resp mesos.Response, err error) {
	func() {
		var n Notification

		state.m.Lock()
		defer state.m.Unlock()
		state.call = call
		state.fn, n = state.fn(ctx, state)
		if n != withoutNotification {
			state.client.notify(n)
		}
		resp, err = state.resp, state.err
	}()

	if debug && err != nil {
		log.Print(*call, err)
	}

	return
}

var withoutNotification = Notification{}
