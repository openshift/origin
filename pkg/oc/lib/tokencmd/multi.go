package tokencmd

import (
	"net/http"

	"k8s.io/klog"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

var _ = ChallengeHandler(&MultiHandler{})

// MultiHandler manages a series of authentication challenges
// it is single-use only, and not thread-safe
type MultiHandler struct {
	// handler holds the selected handler.
	// automatically populated with the first handler to successfully respond to HandleChallenge(),
	// and used exclusively by CanHandle() and HandleChallenge() from that point forward.
	handler ChallengeHandler

	// possibleHandlers holds handlers that could handle subsequent challenges.
	// filtered down during HandleChallenge() by calling CanHandle() on each item.
	possibleHandlers []ChallengeHandler

	// allHandlers holds all handlers, for purposes of delegating Release() calls
	allHandlers []ChallengeHandler
}

func NewMultiHandler(handlers ...ChallengeHandler) ChallengeHandler {
	return &MultiHandler{
		possibleHandlers: handlers,
		allHandlers:      handlers,
	}
}

func (h *MultiHandler) CanHandle(headers http.Header) bool {
	// If we've already selected a handler, it alone can decide whether we can handle the current request
	if h.handler != nil {
		return h.handler.CanHandle(headers)
	}

	// Otherwise, return true if any of our handlers can handle this request
	for _, handler := range h.possibleHandlers {
		if handler.CanHandle(headers) {
			return true
		}
	}

	return false
}

func (h *MultiHandler) HandleChallenge(requestURL string, headers http.Header) (http.Header, bool, error) {
	// If we've already selected a handler, it alone can handle all subsequent challenges (don't change horses in mid-stream)
	if h.handler != nil {
		return h.handler.HandleChallenge(requestURL, headers)
	}

	// Otherwise, filter our list of handlers to the ones that can handle this request
	applicable := []ChallengeHandler{}
	for _, handler := range h.possibleHandlers {
		if handler.CanHandle(headers) {
			applicable = append(applicable, handler)
		}
	}
	h.possibleHandlers = applicable

	// Then select the first available handler that successfully handles the request
	var (
		retryHeaders http.Header
		retry        bool
		err          error
	)
	for i, handler := range h.possibleHandlers {
		retryHeaders, retry, err = handler.HandleChallenge(requestURL, headers)

		if err != nil {
			klog.V(5).Infof("handler[%d] error: %v", i, err)
		}
		// If the handler successfully handled the challenge, or we have no other options, select it as our handler
		if err == nil || i == len(h.possibleHandlers)-1 {
			h.handler = handler
			return retryHeaders, retry, err
		}
	}

	return nil, false, apierrs.NewUnauthorized("unhandled challenge")
}

func (h *MultiHandler) CompleteChallenge(requestURL string, headers http.Header) error {
	if h.handler != nil {
		return h.handler.CompleteChallenge(requestURL, headers)
	}
	return nil
}

func (h *MultiHandler) Release() error {
	var errs []error
	for _, handler := range h.allHandlers {
		if err := handler.Release(); err != nil {
			errs = append(errs, err)
		}
	}
	return utilerrors.NewAggregate(errs)
}
