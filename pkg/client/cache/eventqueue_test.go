/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
)

func TestEventQueue_basic(t *testing.T) {
	q := NewEventQueue()

	q.Add("boo", 2)
	q.Add("foo", 10)
	q.Add("bar", 1)
	q.Update("foo", 11)
	q.Update("foo", 13)
	q.Delete("bar")
	q.Add("zab", 30)

	event, thing := q.Pop()

	if thing != 2 {
		t.Fatalf("expected %v, got %v", 2, thing)
	}

	if event != watch.Added {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}

	q.Update("boo", 3)

	event, thing = q.Pop()

	if thing != 13 {
		t.Fatalf("expected %v, got %v", 13, thing)
	}

	if event != watch.Added {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}

	event, thing = q.Pop()

	if thing != 30 {
		t.Fatalf("expected %v, got %v", 30, thing)
	}

	if event != watch.Added {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}

	event, thing = q.Pop()

	if thing != 3 {
		t.Fatalf("expected %v, got %v", 3, thing)
	}

	if event != watch.Modified {
		t.Fatalf("expected %s, got %s", watch.Modified, event)
	}
}

func TestEventQueue_initialEventIsDelete(t *testing.T) {
	q := NewEventQueue()

	q.Replace(map[string]interface{}{
		"foo": 2,
	})

	q.Delete("foo")

	event, thing := q.Pop()

	if thing != 2 {
		t.Fatalf("expected %v, got %v", 2, thing)
	}

	if event != watch.Deleted {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}
}

func TestEventQueue_compressAddDelete(t *testing.T) {
	q := NewEventQueue()

	q.Add("foo", 10)
	q.Delete("foo")
	q.Add("zab", 30)

	event, thing := q.Pop()

	if thing != 30 {
		t.Fatalf("expected %v, got %v", 30, thing)
	}

	if event != watch.Added {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}
}

func TestEventQueue_compressAddUpdate(t *testing.T) {
	q := NewEventQueue()

	q.Add("foo", 10)
	q.Update("foo", 11)

	event, thing := q.Pop()

	if thing != 11 {
		t.Fatalf("expected %v, got %v", 11, thing)
	}

	if event != watch.Added {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}
}

func TestEventQueue_compressTwoUpdates(t *testing.T) {
	q := NewEventQueue()

	q.Replace(map[string]interface{}{
		"foo": 2,
	})

	q.Update("foo", 3)
	q.Update("foo", 4)

	event, thing := q.Pop()

	if thing != 4 {
		t.Fatalf("expected %v, got %v", 4, thing)
	}

	if event != watch.Modified {
		t.Fatalf("expected %s, got %s", watch.Modified, event)
	}
}

func TestEventQueue_compressUpdateDelete(t *testing.T) {
	q := NewEventQueue()

	q.Replace(map[string]interface{}{
		"foo": 2,
	})

	q.Update("foo", 3)
	q.Delete("foo")

	event, thing := q.Pop()

	if thing != 3 {
		t.Fatalf("expected %v, got %v", 3, thing)
	}

	if event != watch.Deleted {
		t.Fatalf("expected %s, got %s", watch.Deleted, event)
	}
}
