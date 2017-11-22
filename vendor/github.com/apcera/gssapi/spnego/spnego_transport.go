// Copyright 2013-2015 Apcera Inc. All rights reserved.

package spnego

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/apcera/gssapi"
)

const negotiateScheme = "Negotiate"

// AddSPNEGONegotiate adds a Negotiate header with the value of a serialized
// token to an http header.
func AddSPNEGONegotiate(h http.Header, name string, token *gssapi.Buffer) {
	if name == "" {
		return
	}

	v := negotiateScheme
	if token.Length() != 0 {
		data := token.Bytes()
		v = v + " " + base64.StdEncoding.EncodeToString(data)
	}
	h.Set(name, v)
}

// CheckSPNEGONegotiate checks for the presence of a Negotiate header. If
// present, we return a gssapi Token created from the header value sent to us.
func CheckSPNEGONegotiate(lib *gssapi.Lib, h http.Header, name string) (bool, *gssapi.Buffer) {
	var err error
	defer func() {
		if err != nil {
			lib.Debug(fmt.Sprintf("CheckSPNEGONegotiate: %v", err))
		}
	}()

	for _, header := range h[http.CanonicalHeaderKey(name)] {
		if len(header) < len(negotiateScheme) {
			continue
		}
		if !strings.EqualFold(header[:len(negotiateScheme)], negotiateScheme) {
			continue
		}

		// Remove the "Negotiate" prefix
		normalizedToken := header[len(negotiateScheme):]
		// Trim leading and trailing whitespace
		normalizedToken = strings.TrimSpace(normalizedToken)
		// Remove internal whitespace (some servers insert whitespace every 76 chars)
		normalizedToken = strings.Replace(normalizedToken, " ", "", -1)
		// Pad to a multiple of 4 chars for base64 (some servers strip trailing padding)
		if unpaddedChars := len(normalizedToken) % 4; unpaddedChars != 0 {
			normalizedToken += strings.Repeat("=", 4-unpaddedChars)
		}

		tbytes, err := base64.StdEncoding.DecodeString(normalizedToken)
		if err != nil {
			continue
		}

		if len(tbytes) == 0 {
			return true, nil
		}

		token, err := lib.MakeBufferBytes(tbytes)
		if err != nil {
			continue
		}
		return true, token
	}

	return false, nil
}
