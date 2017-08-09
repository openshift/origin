package maxconnections

import (
	"reflect"
	"sync"
	"testing"
)

type countM map[interface{}]int

type counter struct {
	mu sync.Mutex
	m  countM
}

func newCounter() *counter {
	return &counter{
		m: make(countM),
	}
}

func (c *counter) Add(key interface{}, delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] += delta
}

func (c *counter) Values() countM {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := make(map[interface{}]int)
	for k, v := range c.m {
		m[k] = v
	}
	return m
}

func (c *counter) Equal(m countM) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range m {
		if c.m[k] != v {
			return false
		}
	}
	for k, v := range c.m {
		if _, ok := m[k]; !ok && v != 0 {
			return false
		}
	}
	return true
}

func TestCounter(t *testing.T) {
	c := newCounter()
	c.Add(100, 1)
	c.Add(200, 2)
	c.Add(300, 3)
	if expected := (countM{100: 1, 200: 2, 300: 3}); !reflect.DeepEqual(c.m, expected) {
		t.Fatalf("c.m = %v, want %v", c.m, expected)
	}
	if expected := (countM{100: 1, 200: 2, 300: 3}); !c.Equal(expected) {
		t.Fatalf("counter(%v).Equal(%v) is false, want true", c.m, expected)
	}

	c.Add(200, -2)
	if expected := (countM{100: 1, 200: 0, 300: 3}); !c.Equal(expected) {
		t.Fatalf("counter(%v).Equal(%v) is false, want true", c.m, expected)
	}
	if expected := (countM{100: 1, 300: 3}); !c.Equal(expected) {
		t.Fatalf("counter(%v).Equal(%v) is false, want true", c.m, expected)
	}
	if expected := (countM{100: 1, 300: 3, 400: 0}); !c.Equal(expected) {
		t.Fatalf("counter(%v).Equal(%v) is false, want true", c.m, expected)
	}

	if expected := (countM{100: 1}); c.Equal(expected) {
		t.Fatalf("counter(%v).Equal(%v) is true, want false", c.m, expected)
	}
	if expected := (countM{100: 1, 300: 3, 400: 4}); c.Equal(expected) {
		t.Fatalf("counter(%v).Equal(%v) is true, want false", c.m, expected)
	}
}
