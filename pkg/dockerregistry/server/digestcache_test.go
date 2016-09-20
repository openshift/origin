package server

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/util/clock"
	"k8s.io/kubernetes/pkg/util/diff"
)

const (
	allowedDeviation = time.Millisecond * 10

	ttl1m = time.Minute
	ttl5m = time.Minute * 5
	ttl8m = time.Minute * 8
)

func TestRepositoryBucketAdd(t *testing.T) {
	now := time.Now()
	clock := clock.NewFakeClock(now)

	generated := make([]bucketEntry, bucketSize)
	for i := 0; i < bucketSize; i++ {
		generated[i] = bucketEntry{
			repository: fmt.Sprintf("gen%d", i),
			evictOn:    now.Add(ttl5m),
		}
	}

	for _, tc := range []struct {
		name            string
		ttl             time.Duration
		repos           []string
		entries         []bucketEntry
		expectedEntries []bucketEntry
	}{
		{
			name:  "no existing entries",
			ttl:   ttl5m,
			repos: []string{"a", "b"},
			expectedEntries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name: "no entries to add",
			ttl:  ttl5m,
			entries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name:  "add few new entries",
			ttl:   ttl8m,
			repos: []string{"bmw", "audi"},
			entries: []bucketEntry{
				{
					repository: "skoda",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "ford",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "skoda",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "ford",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "bmw",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "audi",
					evictOn:    now.Add(ttl8m),
				},
			},
		},

		{
			name:  "add existing entry with single item",
			ttl:   ttl8m,
			repos: []string{"apple"},
			entries: []bucketEntry{
				{repository: "apple", evictOn: now.Add(ttl5m)},
			},
			expectedEntries: []bucketEntry{
				{repository: "apple", evictOn: now.Add(ttl8m)},
			},
		},

		{
			name:  "add existing entry with higher ttl",
			ttl:   ttl8m,
			repos: []string{"apple"},
			entries: []bucketEntry{
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl8m),
				},
			},
		},

		{
			name:  "add existing entry with lower ttl",
			ttl:   ttl5m,
			repos: []string{"orange"},
			entries: []bucketEntry{
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
			},
		},

		{
			name:  "add new entry with eviction",
			ttl:   ttl5m,
			repos: []string{"banana"},
			entries: []bucketEntry{
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "apple",
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "banana",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name:  "all stale",
			ttl:   ttl5m,
			repos: []string{"banana"},
			entries: []bucketEntry{
				{
					repository: "orange",
				},
				{
					repository: "apple",
				},
				{
					repository: "pear",
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "banana",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name:  "add multiple entries with middle ttl",
			ttl:   ttl5m,
			repos: []string{"apple", "banana", "peach", "orange"},
			entries: []bucketEntry{
				{
					repository: "melon",
				},
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl1m),
				},
				{
					repository: "plum",
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "pear",
					evictOn:    now.Add(ttl1m),
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "banana",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "peach",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
			},
		},

		{
			name:    "over bucket size",
			ttl:     ttl1m,
			repos:   []string{"new1", generated[2].repository, "new2", generated[4].repository},
			entries: generated,
			expectedEntries: append(
				append([]bucketEntry{generated[3]}, generated[5:bucketSize]...),
				bucketEntry{repository: "new1", evictOn: now.Add(ttl1m)},
				generated[2],
				bucketEntry{repository: "new2", evictOn: now.Add(ttl1m)},
				generated[4]),
		},
	} {
		b := repositoryBucket{
			clock: clock,
			list:  tc.entries,
		}
		b.Add(tc.ttl, tc.repos...)

		if len(b.list) != len(tc.expectedEntries) {
			t.Errorf("[%s] got unexpected number of entries in bucket: %d != %d", tc.name, len(b.list), len(tc.expectedEntries))
		}
		for i := 0; i < len(b.list); i++ {
			if i >= len(tc.expectedEntries) {
				t.Errorf("[%s] index=%d got unexpected entry: %#+v", tc.name, i, b.list[i])
				continue
			}
			a, b := b.list[i], tc.expectedEntries[i]
			if !bucketEntriesEqual(a, b) {
				t.Errorf("[%s] index=%d got unexpected entry: %#+v != %#+v", tc.name, i, a, b)
			}
		}
		for i := len(b.list); i < len(tc.expectedEntries); i++ {
			if i >= len(tc.expectedEntries) {
				t.Errorf("[%s] index=%d missing expected entry %#+v", tc.name, i, tc.expectedEntries[i])
			}
		}
	}
}

func TestRepositoryBucketAddOversize(t *testing.T) {
	clock := clock.NewFakeClock(time.Now())

	b := repositoryBucket{
		clock: clock,
	}

	i := 0
	for ; i < bucketSize; i++ {
		ttl := time.Duration(uint64(ttl5m) * uint64(i))
		b.Add(ttl, fmt.Sprintf("%d", i))
	}
	if len(b.list) != bucketSize {
		t.Fatalf("unexpected number of items: %d != %d", len(b.list), bucketSize)
	}

	// make first three stale
	clock.Step(ttl5m * 3)
	if !b.Has("3") {
		t.Fatalf("bucket does not contain repository 3")
	}
	if len(b.list) != bucketSize-3 {
		t.Fatalf("unexpected number of items: %d != %d", len(b.list), bucketSize-3)
	}

	// add few repos one by one
	for ; i < bucketSize+5; i++ {
		ttl := time.Duration(uint64(ttl5m) * uint64(i))
		b.Add(ttl, fmt.Sprintf("%d", i))
	}
	if len(b.list) != bucketSize {
		t.Fatalf("unexpected number of items: %d != %d", len(b.list), bucketSize)
	}

	// add few repos at once
	newRepos := []string{}
	for ; i < bucketSize+10; i++ {
		newRepos = append(newRepos, fmt.Sprintf("%d", i))
	}
	b.Add(ttl5m, newRepos...)
	if len(b.list) != bucketSize {
		t.Fatalf("unexpected number of items: %d != %d", len(b.list), bucketSize)
	}

	for j := 0; j < bucketSize; j++ {
		expected := fmt.Sprintf("%d", i-bucketSize+j)
		if b.list[j].repository != expected {
			t.Fatalf("unexpected repository on index %d: %s != %s", j, b.list[j].repository, expected)
		}
	}
}

func TestRepositoryBucketRemove(t *testing.T) {
	now := time.Now()
	clock := clock.NewFakeClock(now)

	for _, tc := range []struct {
		name            string
		repos           []string
		entries         []bucketEntry
		expectedEntries []bucketEntry
	}{
		{
			name:  "no existing entries",
			repos: []string{"a", "b"},
		},

		{
			name:  "no matching entries",
			repos: []string{"c", "d"},
			entries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name: "no entries to remove",
			entries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name:  "remove one matching",
			repos: []string{"bmw", "skoda"},
			entries: []bucketEntry{
				{
					repository: "skoda",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "ford",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "ford",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name:  "remove existing entry with single item",
			repos: []string{"apple"},
			entries: []bucketEntry{
				{repository: "apple", evictOn: now.Add(ttl5m)},
			},
			expectedEntries: []bucketEntry{},
		},

		{
			name:  "remove, no eviction",
			repos: []string{"pear"},
			entries: []bucketEntry{
				{
					repository: "orange",
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "orange",
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
			},
		},

		{
			name:  "remove multiple matching",
			repos: []string{"orange", "apple"},
			entries: []bucketEntry{
				{
					repository: "orange",
					evictOn:    now.Add(ttl8m),
				},
				{
					repository: "apple",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedEntries: []bucketEntry{
				{
					repository: "pear",
					evictOn:    now.Add(ttl5m),
				},
			},
		},
	} {
		b := repositoryBucket{
			clock: clock,
			list:  tc.entries,
		}
		b.Remove(tc.repos...)

		if len(b.list) != len(tc.expectedEntries) {
			t.Errorf("[%s] got unexpected number of entries in bucket: %d != %d", tc.name, len(b.list), len(tc.expectedEntries))
		}
		for i := 0; i < len(b.list); i++ {
			if i >= len(tc.expectedEntries) {
				t.Errorf("[%s] index=%d got unexpected entry: %#+v", tc.name, i, b.list[i])
				continue
			}
			a, b := b.list[i], tc.expectedEntries[i]
			if !bucketEntriesEqual(a, b) {
				t.Errorf("[%s] index=%d got unexpected entry: %#+v != %#+v", tc.name, i, a, b)
			}
		}
		for i := len(b.list); i < len(tc.expectedEntries); i++ {
			if i >= len(tc.expectedEntries) {
				t.Errorf("[%s] index=%d missing expected entry %#+v", tc.name, i, tc.expectedEntries[i])
			}
		}
	}
}

func TestRepositoryBucketCopy(t *testing.T) {
	now := time.Now()
	clock := clock.NewFakeClock(now)

	ttl5m := time.Minute * 5
	for _, tc := range []struct {
		name          string
		entries       []bucketEntry
		expectedRepos []string
	}{
		{
			name:          "no entry",
			expectedRepos: []string{},
		},

		{
			name: "one stale entry",
			entries: []bucketEntry{
				{
					repository: "1",
				},
			},
			expectedRepos: []string{},
		},

		{
			name: "two entries",
			entries: []bucketEntry{
				{
					repository: "a",
					evictOn:    now.Add(ttl5m),
				},
				{
					repository: "b",
					evictOn:    now.Add(ttl5m),
				},
			},
			expectedRepos: []string{"a", "b"},
		},
	} {
		b := repositoryBucket{
			clock: clock,
			list:  tc.entries,
		}
		result := b.Copy()

		if !reflect.DeepEqual(result, tc.expectedRepos) {
			t.Errorf("[%s] got unexpected repo list: %s", tc.name, diff.ObjectGoPrintDiff(result, tc.expectedRepos))
		}
	}
}

func bucketEntriesEqual(a, b bucketEntry) bool {
	if a.repository != b.repository || a.evictOn != b.evictOn {
		return false
	}
	return true
}
