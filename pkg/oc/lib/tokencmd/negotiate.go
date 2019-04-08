package tokencmd

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"k8s.io/klog"
)

// Negotiator defines the minimal interface needed to interact with GSSAPI to perform a negotiate challenge/response
type Negotiator interface {
	// Load gives the negotiator a chance to load any resources needed to handle a challenge/response sequence.
	// It may be invoked multiple times. If an error is returned, InitSecContext and IsComplete are not called, but Release() is.
	Load() error
	// InitSecContext returns the response token for a Negotiate challenge token from a given URL,
	// or an error if no response token could be obtained or the incoming token is invalid.
	InitSecContext(requestURL string, challengeToken []byte) (tokenToSend []byte, err error)
	// IsComplete returns true if the negotiator is satisfied with the negotiation.
	// This typically means gssapi returned GSS_S_COMPLETE to an initSecContext call.
	IsComplete() bool
	// Release gives the negotiator a chance to release any resources held during a challenge/response sequence.
	// It is always invoked, even in cases where no challenges were received or handled.
	Release() error
}

// NegotiateChallengeHandler manages a challenge negotiation session
// it is single-host, single-use only, and not thread-safe
type NegotiateChallengeHandler struct {
	negotiator Negotiator
}

func NewNegotiateChallengeHandler(negotiator Negotiator) ChallengeHandler {
	return &NegotiateChallengeHandler{negotiator: negotiator}
}

func (c *NegotiateChallengeHandler) CanHandle(headers http.Header) bool {
	// Make sure this is a negotiate request
	if isNegotiate, _, err := getNegotiateToken(headers); err != nil || !isNegotiate {
		return false
	}
	// Make sure our negotiator can initialize
	if err := c.negotiator.Load(); err != nil {
		return false
	}
	return true
}

func (c *NegotiateChallengeHandler) HandleChallenge(requestURL string, headers http.Header) (http.Header, bool, error) {
	// Get incoming token
	_, incomingToken, err := getNegotiateToken(headers)
	if err != nil {
		return nil, false, err
	}

	// Process the token
	outgoingToken, err := c.negotiator.InitSecContext(requestURL, incomingToken)
	if err != nil {
		klog.V(5).Infof("InitSecContext returned error: %v", err)
		return nil, false, err
	}

	// Build the response headers
	responseHeaders := http.Header{}
	responseHeaders.Set("Authorization", "Negotiate "+base64.StdEncoding.EncodeToString(outgoingToken))
	return responseHeaders, true, nil
}

func (c *NegotiateChallengeHandler) CompleteChallenge(requestURL string, headers http.Header) error {
	if c.negotiator.IsComplete() {
		return nil
	}
	klog.V(5).Infof("continue needed")

	// Get incoming token
	isNegotiate, incomingToken, err := getNegotiateToken(headers)
	if err != nil {
		return err
	}
	if !isNegotiate {
		return errors.New("client requires final negotiate token, none provided")
	}

	// Process the token
	_, err = c.negotiator.InitSecContext(requestURL, incomingToken)
	if err != nil {
		klog.V(5).Infof("InitSecContext returned error during final negotiation: %v", err)
		return err
	}
	if !c.negotiator.IsComplete() {
		return errors.New("InitSecContext did not indicate final negotiation completed")
	}
	return nil
}

func (c *NegotiateChallengeHandler) Release() error {
	return c.negotiator.Release()
}

const negotiateScheme = "negotiate"

func getNegotiateToken(headers http.Header) (bool, []byte, error) {
	for _, challengeHeader := range headers[http.CanonicalHeaderKey("WWW-Authenticate")] {
		// TODO: handle WWW-Authenticate headers containing more than one scheme
		caseInsensitiveHeader := strings.ToLower(challengeHeader)
		if caseInsensitiveHeader == negotiateScheme {
			return true, nil, nil
		}
		if strings.HasPrefix(caseInsensitiveHeader, negotiateScheme+" ") {
			payload := challengeHeader[len(negotiateScheme):]
			payload = strings.Replace(payload, " ", "", -1)
			data, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				return false, nil, err
			}
			return true, data, nil
		}
	}
	return false, nil, nil
}
