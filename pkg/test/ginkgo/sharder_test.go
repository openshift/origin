package ginkgo

import (
	"fmt"
	"math"
	"testing"
)

func TestHashSharder_DeterministicSharding(t *testing.T) {
	sharder := &HashSharder{}
	shardCount := 10
	shardID := 3

	// Generate 1000 unique test cases
	var allTests []*testCase
	for i := 0; i < 1000; i++ {
		allTests = append(allTests, &testCase{name: fmt.Sprintf("test-%d", i)})
	}

	// Shard multiple times and ensure determinism
	shardedOnce, err := sharder.Shard(allTests, shardCount, shardID)
	if err != nil {
		t.Fatalf("shard failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		shardedAgain, err := sharder.Shard(allTests, shardCount, shardID)
		if err != nil {
			t.Fatalf("shard repeat %d failed: %v", i, err)
		}

		if len(shardedOnce) != len(shardedAgain) {
			t.Errorf("iteration %d: expected %d tests, got %d", i, len(shardedOnce), len(shardedAgain))
		}

		for j := range shardedOnce {
			if shardedOnce[j].name != shardedAgain[j].name {
				t.Errorf("iteration %d, test %d: expected %s, got %s", i, j, shardedOnce[j].name, shardedAgain[j].name)
			}
		}
	}
}

func TestHashSharder_BalancedSharding(t *testing.T) {
	sharder := &HashSharder{}
	totalShards := 10
	numTests := 12345
	allowedDeviation := 0.10 // 10%

	var allTests []*testCase
	for i := 0; i < numTests; i++ {
		allTests = append(allTests, &testCase{name: fmt.Sprintf("test-%d", i)})
	}

	shardCounts := make([]int, totalShards)

	for shardID := 1; shardID <= totalShards; shardID++ {
		sharded, err := sharder.Shard(allTests, totalShards, shardID)
		if err != nil {
			t.Fatalf("sharding failed for shardID %d: %v", shardID, err)
		}
		shardCounts[shardID-1] = len(sharded)
	}

	expected := float64(numTests) / float64(totalShards)
	for i, count := range shardCounts {
		diff := math.Abs(float64(count) - expected)
		if diff/expected > allowedDeviation {
			t.Errorf("Shard %d has %d tests (expected ~%.0f, deviation %.2f%%)", i+1, count, expected, (diff/expected)*100)
		} else {
			t.Logf("Shard %d has %d tests", i+1, count)
		}
	}
}

func TestHashSharder_DisabledSharding(t *testing.T) {
	sharder := &HashSharder{}
	tests := []*testCase{{name: "test-a"}, {name: "test-b"}}

	sharded, err := sharder.Shard(tests, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sharded) != len(tests) {
		t.Errorf("expected all tests returned when sharding is disabled")
	}
}

func TestHashSharder_InvalidShardID(t *testing.T) {
	sharder := &HashSharder{}
	tests := []*testCase{{name: "test-a"}}

	_, err := sharder.Shard(tests, 10, 11)
	if err == nil {
		t.Errorf("expected error for invalid shard id")
	}
}

func TestHashSharder_EmptyInput(t *testing.T) {
	sharder := &HashSharder{}
	var empty []*testCase

	sharded, err := sharder.Shard(empty, 5, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sharded) != 0 {
		t.Errorf("expected 0 tests, got %d", len(sharded))
	}
}

func TestHashSharder_MoreShardsThanTests(t *testing.T) {
	sharder := &HashSharder{}
	totalShards := 20
	numTests := 10

	var allTests []*testCase
	for i := 0; i < numTests; i++ {
		allTests = append(allTests, &testCase{name: fmt.Sprintf("test-%d", i)})
	}

	seenTests := make(map[string]bool)
	totalAssigned := 0

	for executor := 1; executor <= totalShards; executor++ {
		sharded, err := sharder.Shard(allTests, totalShards, executor)
		if err != nil {
			t.Fatalf("sharding failed: %v", err)
		}

		for _, test := range sharded {
			if seenTests[test.name] {
				t.Errorf("test %s assigned to multiple shards", test.name)
			}
			seenTests[test.name] = true
		}

		totalAssigned += len(sharded)
	}

	if totalAssigned != numTests {
		t.Errorf("total assigned tests = %d; expected %d", totalAssigned, numTests)
	}
}
