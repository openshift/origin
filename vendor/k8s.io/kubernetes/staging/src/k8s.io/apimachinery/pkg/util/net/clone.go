// +build go1.8

package net

import (
	"crypto/tls"
)

// CloneTLSConfig returns a tls.Config with all exported fields except SessionTicketsDisabled and SessionTicketKey copied.
// This makes it safe to call CloneTLSConfig on a config in active use by a server.
func CloneTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		cfgCopy := tls.Config{}
		// Clone() mutates the receiver (!), so also call it on the copy
		// This prevents issues when using reflect.DeepEqual()
		cfgCopy.Clone()
		return &cfgCopy
	}

	cfgCopy := cfg.Clone()
	// Clone() mutates the receiver (!), so also call it on the copy
	// This prevents issues when using reflect.DeepEqual()
	cfgCopy.Clone()
	return cfgCopy
}

