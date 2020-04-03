package perf

import (
	"bytes"
	"io"
	"testing"

	"github.com/cilium/ebpf/internal/unix"
)

func TestRingBufferReader(t *testing.T) {
	buf := make([]byte, 2)

	ring := makeRing(2, 0)
	n, err := ring.Read(buf)
	if err != io.EOF {
		t.Error("Expected io.EOF, got", err)
	}
	if n != 2 {
		t.Errorf("Expected to read 2 bytes, got %d", n)
	}
	if !bytes.Equal(buf, []byte{0, 1}) {
		t.Error("Expected [0, 1], got", buf)
	}
	n, err = ring.Read(buf)
	if err != io.EOF {
		t.Error("Expected io.EOF, got", err)
	}
	if n != 0 {
		t.Error("Expected to read 0 bytes, got", n)
	}

	// Wrapping read
	ring = makeRing(2, 1)
	n, err = io.ReadFull(ring, buf)
	if err != nil {
		t.Error("Error while reading:", err)
	}
	if n != 2 {
		t.Errorf("Expected to read 2 byte, got %d", n)
	}
	if !bytes.Equal(buf, []byte{1, 0}) {
		t.Error("Expected [1, 0], got", buf)
	}
}

func makeRing(size, offset int) *ringReader {
	if size%2 != 0 {
		panic("size must be power of two")
	}

	ring := make([]byte, size)
	for i := range ring {
		ring[i] = byte(i)
	}

	meta := unix.PerfEventMmapPage{
		Data_head: uint64(len(ring) + offset),
		Data_tail: uint64(offset),
		Data_size: uint64(len(ring)),
	}

	return newRingReader(&meta, ring)
}
