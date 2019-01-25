package mesos

import (
	"reflect"
	"testing"
)

func TestNewRanges(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		ns   []uint64
		want Ranges
	}{
		{[]uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, Ranges{{0, 10}}},
		{[]uint64{7, 2, 3, 8, 1, 5, 0, 9, 6, 10, 4}, Ranges{{0, 10}}},
		{[]uint64{0, 0, 1, 1, 2, 2, 3, 3, 4, 4}, Ranges{{0, 4}}},
		{[]uint64{0, 1, 3, 5, 6, 8, 9}, Ranges{{0, 1}, {3, 3}, {5, 6}, {8, 9}}},
		{[]uint64{1}, Ranges{{1, 1}}},
		{[]uint64{}, Ranges{}},
	} {
		if got := NewRanges(tt.ns...); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d, NewRanges(%v): got: %v, want: %v", i, tt.ns, got, tt.want)
		}
	}
}

func TestNewPortRanges(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		Ranges
		want Ranges
	}{
		{Ranges{{2, 0}, {3, 10}}, Ranges{{0, 10}}},
		{Ranges{{0, 2}, {3, 10}}, Ranges{{0, 10}}},
		{Ranges{{0, 2}, {1, 10}}, Ranges{{0, 10}}},
		{Ranges{{0, 2}, {4, 10}}, Ranges{{0, 2}, {4, 10}}},
		{Ranges{{10, 0}}, Ranges{{0, 10}}},
		{Ranges{}, Ranges{}},
		{nil, Ranges{}},
	} {
		offer := &Offer{Resources: []Resource{tt.resource("ports")}}
		if got := NewPortRanges(offer); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d, NewPortRanges(%v): got: %v, want: %v", i, tt.Ranges, got, tt.want)
		}
	}
}

func TestRanges_Size(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		Ranges
		want uint64
	}{
		{Ranges{}, 0},
		{Ranges{{0, 1}, {2, 10}}, 11},
		{Ranges{{0, 1}, {2, 10}, {11, 100}}, 101},
		{Ranges{{1, 1}, {2, 10}, {11, 100}}, 100},
	} {
		if got := tt.Size(); got != tt.want {
			t.Errorf("test #%d, Size(%v): got: %v, want: %v", i, tt.Ranges, got, tt.want)
		}
	}
}

func TestRanges_Squash(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		Ranges
		want Ranges
	}{
		{Ranges{}, Ranges{}},
		{Ranges{{0, 1}}, Ranges{{0, 1}}},
		{Ranges{{0, 2}, {1, 5}, {2, 10}}, Ranges{{0, 10}}},
		{Ranges{{0, 2}, {2, 5}, {5, 10}}, Ranges{{0, 10}}},
		{Ranges{{0, 2}, {3, 5}, {6, 10}}, Ranges{{0, 10}}},
		{Ranges{{0, 2}, {4, 11}, {6, 10}}, Ranges{{0, 2}, {4, 11}}},
		{Ranges{{0, 2}, {4, 5}, {6, 7}, {8, 10}}, Ranges{{0, 2}, {4, 10}}},
		{Ranges{{0, 2}, {4, 6}, {8, 10}}, Ranges{{0, 2}, {4, 6}, {8, 10}}},
		{Ranges{{0, 1}, {2, 5}, {4, 8}}, Ranges{{0, 8}}},
	} {
		if got := tt.Squash(); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d, Squash(%v): got: %v, want: %v", i, tt.Ranges, got, tt.want)
		}
	}
}

func TestRanges_Search(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		Ranges
		n    uint64
		want int
	}{
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 0, 0},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 1, 0},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 2, 0},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 3, 1},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 4, 1},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 5, 1},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 6, -1},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 7, 2},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 8, 2},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 9, 2},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 10, 2},
		{Ranges{{0, 2}, {3, 5}, {7, 10}}, 11, -1},
		{Ranges{{0, 2}, {4, 4}, {5, 10}}, 4, 1},
	} {
		if got := tt.Search(tt.n); got != tt.want {
			t.Errorf("test #%d: Search(%v, %v): got: %v, want: %v", i, tt.Ranges, tt.n, got, tt.want)
		}
	}
}

func TestRanges_Partition(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		Ranges
		n     uint64
		want  Ranges
		found bool
	}{
		{Ranges{}, 0, Ranges{}, false},
		{Ranges{{0, 10}, {12, 20}}, 100, Ranges{{0, 10}, {12, 20}}, false},
		{Ranges{{0, 10}, {12, 20}}, 0, Ranges{{1, 10}, {12, 20}}, true},
		{Ranges{{0, 10}, {12, 20}}, 13, Ranges{{0, 10}, {12, 12}, {14, 20}}, true},
		{Ranges{{0, 10}, {12, 20}}, 5, Ranges{{0, 4}, {6, 10}, {12, 20}}, true},
		{Ranges{{0, 10}, {12, 20}}, 19, Ranges{{0, 10}, {12, 18}, {20, 20}}, true},
		{Ranges{{0, 10}, {12, 20}}, 10, Ranges{{0, 9}, {12, 20}}, true},
		{Ranges{{0, 10}, {12, 12}, {14, 20}}, 12, Ranges{{0, 10}, {14, 20}}, true},
	} {
		if got, found := tt.Partition(tt.n); !reflect.DeepEqual(got, tt.want) || found != tt.found {
			t.Errorf("test #%d: Partition(%v, %v): got: (%v, %t), want: (%v, %t)", i, tt.Ranges, tt.n, got, found, tt.want, tt.found)
		}
	}
}

func TestRanges_MinMax(t *testing.T) {
	t.Parallel()

	for i, tt := range []struct {
		Ranges
		min, max uint64
	}{
		{Ranges{{1, 10}, {100, 1000}}, 1, 1000},
		{Ranges{{0, 10}, {12, 20}}, 0, 20},
		{Ranges{{5, 10}}, 5, 10},
		{Ranges{{0, 0}}, 0, 0},
	} {
		if got, want := tt.Min(), tt.min; got != want {
			t.Errorf("test #%d: Min(%v): got: %d, want: %d", i, tt.Ranges, got, want)
		}
		if got, want := tt.Max(), tt.max; got != want {
			t.Errorf("test #%d: Max(%v): got: %d, want: %d", i, tt.Ranges, got, want)
		}
	}
}
