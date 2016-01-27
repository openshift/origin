package controller

import (
	"reflect"
	"testing"

	kutil "k8s.io/kubernetes/pkg/util"
)

func TestScheduler(t *testing.T) {
	keys := []string{}
	s := NewScheduler(2, kutil.NewFakeRateLimiter(), func(key, value interface{}) {
		keys = append(keys, key.(string))
	})

	for i := 0; i < 6; i++ {
		s.RunOnce()
		if len(keys) > 0 {
			t.Fatal(keys)
		}
		if s.position != (i+1)%3 {
			t.Fatal(s.position)
		}
	}

	s.Add("first", "test")
	found := false
	for i, buckets := range s.buckets {
		if _, ok := buckets["first"]; ok {
			found = true
		} else {
			continue
		}
		if i == s.position {
			t.Fatal("should not insert into current bucket")
		}
	}
	if !found {
		t.Fatal("expected to find key in a bucket")
	}

	for i := 0; i < 10; i++ {
		s.Delay("first")
		if _, ok := s.buckets[(s.position-1+len(s.buckets))%len(s.buckets)]["first"]; !ok {
			t.Fatal("key was not in the last bucket")
		}
	}

	s.RunOnce()
	if len(keys) != 0 {
		t.Fatal(keys)
	}
	s.RunOnce()
	if !reflect.DeepEqual(keys, []string{"first"}) {
		t.Fatal(keys)
	}
}

func TestSchedulerAdd(t *testing.T) {
	s := NewScheduler(3, kutil.NewFakeRateLimiter(), func(key, value interface{}) {})
	s.Add("first", "other")
	if s.buckets[3]["first"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("second", "other")
	if s.buckets[2]["second"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("third", "other")
	if s.buckets[1]["third"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("fourth", "other")
	if s.buckets[3]["fourth"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("fifth", "other")
	if s.buckets[2]["fifth"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Remove("third", "other")
	s.Add("sixth", "other")
	if s.buckets[1]["sixth"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
}

func TestSchedulerRemove(t *testing.T) {
	s := NewScheduler(2, kutil.NewFakeRateLimiter(), func(key, value interface{}) {})
	s.Add("test", "other")
	if s.Remove("test", "value") {
		t.Fatal(s)
	}
	if !s.Remove("test", "other") {
		t.Fatal(s)
	}
	if s.Len() != 0 {
		t.Fatal(s)
	}
	s.Add("test", "other")
	s.Add("test", "new")
	if s.Len() != 1 {
		t.Fatal(s)
	}
	if s.Remove("test", "other") {
		t.Fatal(s)
	}
	if !s.Remove("test", "new") {
		t.Fatal(s)
	}
}
