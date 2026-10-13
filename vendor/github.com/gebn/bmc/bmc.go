// Package bmc implements an IPMI v1.5/2.0 remote console. pkg/ipmi provides the
// layers; this package makes IPMI work in Go.
package bmc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gebn/bmc/internal/pkg/transport"
	"github.com/gebn/bmc/pkg/ipmi"

	"github.com/google/gopacket"
)

var (
	serializeOptions = gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	namespace = "bmc"
)

type dialConfig struct {
	timeout time.Duration
}

type DialConfigOption func(c *dialConfig)

func WithTimeout(t time.Duration) DialConfigOption {
	return func(c *dialConfig) {
		c.timeout = t
	}
}

// Dial is currently an alias for DialV2. When IPMI v1.5 is implemented, this
// will query the BMC for IPMI v2.0 capability. If it supports IPMI v2.0, a
// V2SessionlessTransport will be returned, otherwise a V1SessionlessTransport
// will be returned. If you know the BMC's capabilities, or need a specific
// feature (e.g. DCMI), use the DialV*() functions instead, which expose
// additional information and functionality.
func Dial(_ context.Context, addr string, opts ...DialConfigOption) (SessionlessTransport, error) {
	return DialV2(addr, opts...)
}

// DialV2 establishes a new IPMI v2.0 connection with the supplied BMC. The
// address is of the form IP[:port] (IPv6 must be enclosed in square brackets).
// Use this if you know the BMC supports IPMI v2.0 and/or require DCMI
// functionality. Note v4 is preferred to v6 if a hostname is passed returning
// both A and AAAA records.
func DialV2(addr string, opts ...DialConfigOption) (*V2SessionlessTransport, error) {
	v2ConnectionOpenAttempts.Inc()
	t, err := newTransport(addr)
	if err != nil {
		v2ConnectionOpenFailures.Inc()
		return nil, err
	}
	v2ConnectionsOpen.Inc()
	c := &dialConfig{
		timeout: 1 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	return newV2SessionlessTransport(t, c), nil
}

func newV2SessionlessTransport(t transport.Transport, c *dialConfig) *V2SessionlessTransport {
	return &V2SessionlessTransport{
		Transport:     t,
		V2Sessionless: newV2Sessionless(t, c.timeout),
	}
}

func newTransport(addr string) (transport.Transport, error) {
	// default to port 623
	if !strings.Contains(addr, ":") || strings.HasSuffix(addr, "]") {
		addr = addr + ":623"
	}
	return transport.New(addr)
}

// ValidateResponse is a helper to remove some boilerplate error handling from
// SendCommand() calls. It ensures a non-nil error and normal completion code.
// If the error is non-nil, it is returned. If the completion code is
// non-normal, an error is returned containing the actual value.
func ValidateResponse(c ipmi.CompletionCode, err error) error {
	if err != nil {
		return err
	}
	if c != ipmi.CompletionCodeNormal {
		return fmt.Errorf("received non-normal completion code: %v", c)
	}
	return nil
}
