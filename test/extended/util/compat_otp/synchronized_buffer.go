package compat_otp

import (
	"bytes"
	"sync"
)

// SynchronizedBuffer wraps bytes.Buffer with a sync.Mutex for thread-safety.
type SynchronizedBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

// NewSynchronizedBuffer initializes an empty SynchronizedBuffer which is ready to use
func NewSynchronizedBuffer() *SynchronizedBuffer {
	return &SynchronizedBuffer{}
}

func (sb *SynchronizedBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Write(p)
}

func (sb *SynchronizedBuffer) String() string {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.String()
}
