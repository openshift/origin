package ttime

import (
  // "github.com/stretchr/testify/assert"
  "testing"
  "time"
)

func TestFreezingTime(t *testing.T) {
  // freeze time at a specific date/time (eg, test leap-year support!):
  now, err := time.Parse(time.RFC3339, "2012-02-29T00:00:00Z")
  if err != nil {
    panic("date time parse failed")
  }
  Freeze(now)

  if !IsFrozen() {
    t.Error("Time should be frozen here, and was not.")
  }
  if Now().UTC() != now {
    t.Error("Time should still be set to frozen time")
  }
  t.Logf("It is now %v (frozen)", Now().UTC())
  Unfreeze()
  if Now().UTC() == now || IsFrozen() {
    t.Error("Time should no longer be frozen")
  }
  t.Logf("It is now %v (not frozen)", Now().UTC())
}

func TestAfter(t *testing.T) {
  // test frozen functionality.
  start := Now()
  Freeze(Now())
  <-After(10 * time.Millisecond)
  Unfreeze()
  elapsed := Now().Sub(start)
  t.Logf("Took %v", elapsed)
  if elapsed >= 1*time.Millisecond {
    t.Error("Took too long")
  }
  // test original functionality is restored
  start = Now()
  <-After(1 * time.Millisecond)
  elapsed = Now().Sub(start)
  t.Logf("Took %v", elapsed)
  if elapsed > 2*time.Millisecond {
    t.Error("Took too long")
  }
  if elapsed < 1*time.Millisecond {
    t.Error("Went too fast")
  }
}

func TestTick(t *testing.T) {
  // test frozen functionality.
  start := Now()
  Freeze(start)
  c := Tick(10 * time.Millisecond)
  <-c
  <-c
  <-c
  if !start.Add(40 * time.Millisecond).Equal(Now()) {
    t.Errorf("Expected Tick to advance clock, did not. %v != %v", start.Add(40*time.Millisecond), Now())
  }
  Unfreeze()
  elapsed := Now().Sub(start)
  t.Logf("Took %v", elapsed)
  if elapsed >= 1*time.Millisecond {
    t.Error("Took too long")
  }
  // test original functionality is restored
  start = Now()
  c = Tick(1 * time.Millisecond)
  <-c
  <-c
  <-c
  elapsed = Now().Sub(start)
  t.Logf("Took %v", elapsed)
  if elapsed > 4*time.Millisecond {
    t.Error("Took too long")
  }
  if elapsed < 3*time.Millisecond {
    t.Error("Went too fast")
  }
}

func TestSleep(t *testing.T) {
  start := time.Now()
  Freeze(start)
  Sleep(1 * time.Second)
  Unfreeze()
  elapsed := time.Now().Sub(start)
  if elapsed > 1*time.Millisecond {
    t.Error("Took too long")
  }
  start = Now()
  Sleep(1 * time.Millisecond)
  elapsed = Now().Sub(start)
  if elapsed > 2*time.Millisecond {
    t.Error("Took too long")
  }
  if elapsed < 1*time.Millisecond {
    t.Error("Went too fast")
  }
}
