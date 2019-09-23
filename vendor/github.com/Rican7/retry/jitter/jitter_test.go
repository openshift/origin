package jitter

import (
	"math/rand"
	"testing"
	"time"
)

func TestFull(t *testing.T) {
	const seed = 0
	const duration = time.Millisecond

	generator := rand.New(rand.NewSource(seed))

	transformation := Full(generator)

	// Based on constant seed
	expectedDurations := []time.Duration{165505, 393152, 995827, 197794, 376202}

	for _, expected := range expectedDurations {
		result := transformation(duration)

		if result != expected {
			t.Errorf("transformation expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestEqual(t *testing.T) {
	const seed = 0
	const duration = time.Millisecond

	generator := rand.New(rand.NewSource(seed))

	transformation := Equal(generator)

	// Based on constant seed
	expectedDurations := []time.Duration{582752, 696576, 997913, 598897, 688101}

	for _, expected := range expectedDurations {
		result := transformation(duration)

		if result != expected {
			t.Errorf("transformation expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestDeviation(t *testing.T) {
	const seed = 0
	const duration = time.Millisecond
	const factor = 0.5

	generator := rand.New(rand.NewSource(seed))

	transformation := Deviation(generator, factor)

	// Based on constant seed
	expectedDurations := []time.Duration{665505, 893152, 1495827, 697794, 876202}

	for _, expected := range expectedDurations {
		result := transformation(duration)

		if result != expected {
			t.Errorf("transformation expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestNormalDistribution(t *testing.T) {
	const seed = 0
	const duration = time.Millisecond
	const standardDeviation = float64(duration / 2)

	generator := rand.New(rand.NewSource(seed))

	transformation := NormalDistribution(generator, standardDeviation)

	// Based on constant seed
	expectedDurations := []time.Duration{859207, 1285466, 153990, 1099811, 1959759}

	for _, expected := range expectedDurations {
		result := transformation(duration)

		if result != expected {
			t.Errorf("transformation expected to return a %s duration, but received %s instead", expected, result)
		}
	}
}

func TestFallbackNewRandom(t *testing.T) {
	generator := rand.New(rand.NewSource(0))

	if result := fallbackNewRandom(generator); generator != result {
		t.Errorf("result expected to match parameter, received %+v instead", result)
	}

	if result := fallbackNewRandom(nil); nil == result {
		t.Error("recieved unexpected nil result")
	}
}
