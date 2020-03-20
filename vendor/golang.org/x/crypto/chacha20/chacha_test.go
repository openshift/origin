// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chacha20

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
)

func _() {
	// Assert that bufSize is a multiple of blockSize.
	var b [1]byte
	_ = b[bufSize%blockSize]
}

func hexDecode(s string) []byte {
	ss, err := hex.DecodeString(s)
	if err != nil {
		panic(fmt.Sprintf("cannot decode input %#v: %v", s, err))
	}
	return ss
}

// Run the test cases with the input and output in different buffers.
func TestNoOverlap(t *testing.T) {
	for _, c := range testVectors {
		s, _ := NewUnauthenticatedCipher(hexDecode(c.key), hexDecode(c.nonce))
		input := hexDecode(c.input)
		output := make([]byte, len(input))
		s.XORKeyStream(output, input)
		got := hex.EncodeToString(output)
		if got != c.output {
			t.Errorf("length=%v: got %#v, want %#v", len(input), got, c.output)
		}
	}
}

// Run the test cases with the input and output overlapping entirely.
func TestOverlap(t *testing.T) {
	for _, c := range testVectors {
		s, _ := NewUnauthenticatedCipher(hexDecode(c.key), hexDecode(c.nonce))
		data := hexDecode(c.input)
		s.XORKeyStream(data, data)
		got := hex.EncodeToString(data)
		if got != c.output {
			t.Errorf("length=%v: got %#v, want %#v", len(data), got, c.output)
		}
	}
}

// Run the test cases with various source and destination offsets.
func TestUnaligned(t *testing.T) {
	const max = 8 // max offset (+1) to test
	for _, c := range testVectors {
		data := hexDecode(c.input)
		input := make([]byte, len(data)+max)
		output := make([]byte, len(data)+max)
		for i := 0; i < max; i++ { // input offsets
			for j := 0; j < max; j++ { // output offsets
				s, _ := NewUnauthenticatedCipher(hexDecode(c.key), hexDecode(c.nonce))

				input := input[i : i+len(data)]
				output := output[j : j+len(data)]

				copy(input, data)
				s.XORKeyStream(output, input)
				got := hex.EncodeToString(output)
				if got != c.output {
					t.Errorf("length=%v: got %#v, want %#v", len(data), got, c.output)
				}
			}
		}
	}
}

// Run the test cases by calling XORKeyStream multiple times.
func TestStep(t *testing.T) {
	// wide range of step sizes to try and hit edge cases
	steps := [...]int{1, 3, 4, 7, 8, 17, 24, 30, 64, 256}
	rnd := rand.New(rand.NewSource(123))
	for _, c := range testVectors {
		s, _ := NewUnauthenticatedCipher(hexDecode(c.key), hexDecode(c.nonce))
		input := hexDecode(c.input)
		output := make([]byte, len(input))

		// step through the buffers
		i, step := 0, steps[rnd.Intn(len(steps))]
		for i+step < len(input) {
			s.XORKeyStream(output[i:i+step], input[i:i+step])
			if i+step < len(input) && output[i+step] != 0 {
				t.Errorf("length=%v, i=%v, step=%v: output overwritten", len(input), i, step)
			}
			i += step
			step = steps[rnd.Intn(len(steps))]
		}
		// finish the encryption
		s.XORKeyStream(output[i:], input[i:])
		// ensure we tolerate a call with an empty input
		s.XORKeyStream(output[len(output):], input[len(input):])

		got := hex.EncodeToString(output)
		if got != c.output {
			t.Errorf("length=%v: got %#v, want %#v", len(input), got, c.output)
		}
	}
}

func benchmarkChaCha20(b *testing.B, step, count int) {
	tot := step * count
	src := make([]byte, tot)
	dst := make([]byte, tot)
	key := make([]byte, KeySize)
	nonce := make([]byte, NonceSize)
	b.SetBytes(int64(tot))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, _ := NewUnauthenticatedCipher(key, nonce)
		for i := 0; i < tot; i += step {
			c.XORKeyStream(dst[i:], src[i:i+step])
		}
	}
}

func BenchmarkChaCha20(b *testing.B) {
	b.Run("64", func(b *testing.B) {
		benchmarkChaCha20(b, 64, 1)
	})
	b.Run("256", func(b *testing.B) {
		benchmarkChaCha20(b, 256, 1)
	})
	b.Run("10x25", func(b *testing.B) {
		benchmarkChaCha20(b, 10, 25)
	})
	b.Run("4096", func(b *testing.B) {
		benchmarkChaCha20(b, 256, 1)
	})
	b.Run("100x40", func(b *testing.B) {
		benchmarkChaCha20(b, 100, 40)
	})
	b.Run("65536", func(b *testing.B) {
		benchmarkChaCha20(b, 65536, 1)
	})
	b.Run("1000x65", func(b *testing.B) {
		benchmarkChaCha20(b, 1000, 65)
	})
}

func TestHChaCha20(t *testing.T) {
	// See draft-irtf-cfrg-xchacha-00, Section 2.2.1.
	key := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
	nonce := []byte{0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x4a,
		0x00, 0x00, 0x00, 0x00, 0x31, 0x41, 0x59, 0x27}
	expected := []byte{0x82, 0x41, 0x3b, 0x42, 0x27, 0xb2, 0x7b, 0xfe,
		0xd3, 0x0e, 0x42, 0x50, 0x8a, 0x87, 0x7d, 0x73,
		0xa0, 0xf9, 0xe4, 0xd5, 0x8a, 0x74, 0xa8, 0x53,
		0xc1, 0x2e, 0xc4, 0x13, 0x26, 0xd3, 0xec, 0xdc,
	}
	result, err := HChaCha20(key[:], nonce[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(expected, result) {
		t.Errorf("want %x, got %x", expected, result)
	}
}
