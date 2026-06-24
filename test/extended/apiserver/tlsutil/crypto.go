// Package tlsutil provides TLS version and cipher suite helpers for API server tests.
//
// The definitions in this file (versions, supportedVersions, ciphers maps and the
// TLSVersion*, ValidTLSVersions, DefaultTLSVersion, CipherSuite, ValidCipherSuites
// functions) are copied verbatim from:
//
//	github.com/openshift/library-go/pkg/crypto/crypto.go
//
// They were extracted here to avoid a library-go dependency bump that caused test
// instability (see OCPBUGS-76334, origin#30746, origin#30895).
//
// If library-go/pkg/crypto/crypto.go is updated (e.g. new TLS versions, new cipher
// suites, or changed defaults), the corresponding sections below must be synced manually.
// The only function NOT from library-go is DefaultCiphers(), which was added locally.
package tlsutil

import (
	"crypto/tls"
	"fmt"
	"sort"
)

// TLS versions that are known to golang.
// Copied from github.com/openshift/library-go/pkg/crypto/crypto.go
var versions = map[string]uint16{
	"VersionTLS10": tls.VersionTLS10,
	"VersionTLS11": tls.VersionTLS11,
	"VersionTLS12": tls.VersionTLS12,
	"VersionTLS13": tls.VersionTLS13,
}

// TLS versions that are enabled.
// Copied from github.com/openshift/library-go/pkg/crypto/crypto.go
var supportedVersions = map[string]uint16{
	"VersionTLS10": tls.VersionTLS10,
	"VersionTLS11": tls.VersionTLS11,
	"VersionTLS12": tls.VersionTLS12,
	"VersionTLS13": tls.VersionTLS13,
}

func TLSVersion(versionName string) (uint16, error) {
	if len(versionName) == 0 {
		return DefaultTLSVersion(), nil
	}
	if version, ok := versions[versionName]; ok {
		return version, nil
	}
	return 0, fmt.Errorf("unknown tls version %q", versionName)
}

func TLSVersionOrDie(versionName string) uint16 {
	version, err := TLSVersion(versionName)
	if err != nil {
		panic(err)
	}
	return version
}

// ValidTLSVersions returns the build enabled TLS versions.
func ValidTLSVersions() []string {
	validVersions := []string{}
	for k := range supportedVersions {
		validVersions = append(validVersions, k)
	}
	sort.Strings(validVersions)
	return validVersions
}

func DefaultTLSVersion() uint16 {
	// Can't use SSLv3 because of POODLE and BEAST
	// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
	// Can't use TLSv1.1 because of RC4 cipher usage
	return tls.VersionTLS12
}

// ciphers maps IANA cipher suite names to their uint16 identifiers.
// Copied from github.com/openshift/library-go/pkg/crypto/crypto.go
var ciphers = map[string]uint16{
	"TLS_RSA_WITH_RC4_128_SHA":                      tls.TLS_RSA_WITH_RC4_128_SHA,
	"TLS_RSA_WITH_3DES_EDE_CBC_SHA":                 tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA":                  tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	"TLS_RSA_WITH_AES_256_CBC_SHA":                  tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	"TLS_RSA_WITH_AES_128_CBC_SHA256":               tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_RSA_WITH_AES_128_GCM_SHA256":               tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_RSA_WITH_AES_256_GCM_SHA384":               tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_RC4_128_SHA":              tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA":          tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_RC4_128_SHA":                tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	"TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA":           tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	"TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA":            tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	"TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256":       tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256":         tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256":         tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256":       tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384":         tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384":       tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305":          tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305":        tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	"TLS_AES_128_GCM_SHA256":                        tls.TLS_AES_128_GCM_SHA256,
	"TLS_AES_256_GCM_SHA384":                        tls.TLS_AES_256_GCM_SHA384,
	"TLS_CHACHA20_POLY1305_SHA256":                  tls.TLS_CHACHA20_POLY1305_SHA256,
}

func CipherSuite(cipherName string) (uint16, error) {
	if cipher, ok := ciphers[cipherName]; ok {
		return cipher, nil
	}
	return 0, fmt.Errorf("unknown cipher name %q", cipherName)
}

func ValidCipherSuites() []string {
	validCipherSuites := []string{}
	for k := range ciphers {
		validCipherSuites = append(validCipherSuites, k)
	}
	sort.Strings(validCipherSuites)
	return validCipherSuites
}

// DefaultCiphers returns the default cipher suites for TLS connections.
// Aligned with intermediate profile of the 5.7 version of the Mozilla Server
// Side TLS guidelines found at: https://ssl-config.mozilla.org/guidelines/5.7.json
func DefaultCiphers() []uint16 {
	return []uint16{
		// TLS 1.2 cipher suites with ECDHE + AEAD
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,

		// TLS 1.3 cipher suites (not configurable, predetermined by spec)
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
	}
}
