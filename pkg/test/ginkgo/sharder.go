package ginkgo

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

type Sharder interface {
	// Shard selects tests for the given executor and total shards.
	// The returned slice must be deterministic and non-overlapping across shards.
	Shard(allTests []*testCase, shardCount, shardID int) ([]*testCase, error)

	// Name returns the name of the strategy (e.g. "hash", "time-balanced").
	Name() string
}

type HashSharder struct{}

func (h *HashSharder) Shard(allTests []*testCase, shardCount, shardID int) ([]*testCase, error) {
	var selected []*testCase

	start := time.Now()

	log := logrus.WithField("sharder", h.Name()).
		WithField("shardCount", shardCount).
		WithField("shardID", shardID)

	log.Infof("Determining sharding of %d tests", len(allTests))

	if shardID > shardCount {
		return nil, fmt.Errorf("shard %d is greater than %d", shardID, shardCount)
	}

	if shardID == 0 || shardCount == 0 {
		logrus.Warningf("Sharding disabled, returning all tests")
		return allTests, nil
	}

	for _, test := range allTests {
		sum := sha256.Sum256([]byte(test.name))
		val := binary.BigEndian.Uint32(sum[:4])
		shard := (int(val) % shardCount) + 1
		if shard == shardID {
			selected = append(selected, test)
		}
	}

	log.Infof("Completed sharding in %+v, this instance will run %d tests", time.Since(start), len(selected))

	return selected, nil
}

func (h *HashSharder) Name() string { return "hash" }
