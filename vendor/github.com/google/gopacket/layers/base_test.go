// Copyright 2012, Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// This file contains some test helper functions.

package layers

import (
	"github.com/google/gopacket"
	"testing"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func checkLayers(p gopacket.Packet, want []gopacket.LayerType, t *testing.T) {
	layers := p.Layers()
	t.Log("Checking packet layers, want", want)
	for _, l := range layers {
		t.Logf("  Got layer %v, %d bytes, payload of %d bytes", l.LayerType(),
			len(l.LayerContents()), len(l.LayerPayload()))
	}
	t.Log(p)
	if len(layers) < len(want) {
		t.Errorf("  Number of layers mismatch: got %d want %d", len(layers),
			len(want))
		return
	}
	for i, l := range want {
		if l == gopacket.LayerTypePayload {
			// done matching layers
			return
		}

		if layers[i].LayerType() != l {
			t.Errorf("  Layer %d mismatch: got %v want %v", i,
				layers[i].LayerType(), l)
		}
	}
}
