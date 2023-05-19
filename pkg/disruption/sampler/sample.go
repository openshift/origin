package sampler

import (
	"fmt"
	"sort"
	"time"
)

// Sample holds information related to a sample
type Sample struct {
	// ID represents the number that uniquely identifies a given sample
	// from the following ordered sequence
	//  1, 2, 3 ... n
	ID         uint64
	StartedAt  time.Time
	FinishedAt time.Time
	// Err is set if the sample had an error (it failed), otherwise nil
	Err error
}

func (s Sample) String() string {
	return fmt.Sprintf("id=%d, at=%s, duration=%s",
		s.ID, s.StartedAt.Format("01/02 15:04:05.000"), s.FinishedAt.Sub(s.StartedAt).Round(time.Millisecond))
}

var _ sort.Interface = SortedByID{}
var _ sort.Interface = SortedByStartedAt{}

type SortedByID []*Sample

func (s SortedByID) Less(i, j int) bool { return s[i].ID < s[j].ID }
func (s SortedByID) Len() int           { return len(s) }
func (s SortedByID) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type SortedByStartedAt []*Sample

func (s SortedByStartedAt) Less(i, j int) bool { return s[i].StartedAt.Before(s[j].StartedAt) }
func (s SortedByStartedAt) Len() int           { return len(s) }
func (s SortedByStartedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
