package mesos_test

import (
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/mesos/mesos-go/api/v1/lib"
)

func scalar(f float64) *mesos.Value_Scalar {
	return &mesos.Value_Scalar{
		Value: f,
	}
}

func r(b, e uint64) mesos.Value_Range {
	return mesos.Value_Range{
		Begin: b,
		End:   e,
	}
}

func ranges(x ...mesos.Value_Range) *mesos.Value_Ranges {
	if x == nil {
		return nil
	}
	return &mesos.Value_Ranges{Range: x}
}

func set(x ...int) *mesos.Value_Set {
	// we accept ints because it allows us to write test cases that
	// are easier on the eyes
	if x == nil {
		return nil
	}
	y := make([]string, len(x))
	for i, xx := range x {
		y[i] = strconv.Itoa(xx)
	}
	return &mesos.Value_Set{Item: y}
}

func TestValue_Set_Compare(t *testing.T) {
	for i, tc := range []struct {
		left, right *mesos.Value_Set
		want        int
	}{
		{nil, nil, 0},
		{nil, set(0), -1},
		{set(0), nil, 1},
		{set(0), set(0), 0},
		{set(1), set(0), 1},
		{set(0), set(1), 1},
		{set(-1), set(0), 1},
		{set(1), set(-1), 1},
		{set(1), set(1), 0},
		{set(-1), set(-1), 0},
		{set(-1), set(-1, 1), -1},
		{set(1), set(-1, 1), -1},
		{set(0), set(1, -1), 1},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Set)
		preright := proto.Clone(tc.right).(*mesos.Value_Set)
		x := tc.left.Compare(tc.right)

		if x != tc.want {
			t.Errorf("test case %d failed: expected %v instead of %v", i, tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Set_Subtract(t *testing.T) {
	for i, tc := range []struct {
		left, right, want *mesos.Value_Set
	}{
		{nil, nil, set()},
		{nil, set(0), set()},
		{set(0), nil, set(0)},
		{set(0), set(0), set()},
		{set(1), set(0), set(1)},
		{set(0), set(1), set(0)},
		{set(-1), set(0), set(-1)},
		{set(1), set(-1), set(1)},
		{set(1), set(1), set()},
		{set(-1), set(-1), set()},
		{set(1, -1), nil, set(-1, 1)},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Set)
		preright := proto.Clone(tc.right).(*mesos.Value_Set)
		x := tc.left.Subtract(tc.right)

		// Add doesn't return a sorted result, so we sort ourselves for
		// predictable test case results
		sort.Strings(x.GetItem())

		if !x.Equal(tc.want) {
			t.Errorf("test case %d failed: expected %v instead of %v", i, tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Set_Add(t *testing.T) {
	for i, tc := range []struct {
		left, right, want *mesos.Value_Set
	}{
		{nil, nil, set()},
		{nil, set(0), set(0)},
		{set(0), nil, set(0)},
		{set(0), set(0), set(0)},
		{set(1), set(0), set(0, 1)},
		{set(0), set(1), set(0, 1)},
		{set(-1), set(0), set(-1, 0)},
		{set(1), set(-1), set(-1, 1)},
		{set(1), set(1), set(1)},
		{set(-1), set(-1), set(-1)},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Set)
		preright := proto.Clone(tc.right).(*mesos.Value_Set)
		x := tc.left.Add(tc.right)

		// Add doesn't return a sorted result, so we sort ourselves for
		// predictable test case results
		sort.Strings(x.GetItem())

		if !x.Equal(tc.want) {
			t.Errorf("test case %d failed: expected %v instead of %v", i, tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Ranges_Compare(t *testing.T) {
	for i, tc := range []struct {
		left, right *mesos.Value_Ranges
		want        int
	}{
		{nil, nil, 0},
		{nil, ranges(r(0, 0)), -1},
		{ranges(r(0, 0)), nil, 1},
		{ranges(r(0, 0)), ranges(r(0, 0)), 0},
		{ranges(r(0, 1)), ranges(r(0, 0)), 1},
		{ranges(r(0, 0)), ranges(r(0, 1)), -1},
		{ranges(r(0, 1)), ranges(r(1, 1)), 1},
		{ranges(r(0, 1)), ranges(r(1, 2)), 1},
		{ranges(r(3, 4), r(0, 1)), ranges(r(1, 2)), 1},
		{ranges(r(1, 2)), ranges(r(2, 6), r(3, 4), r(0, 1)), -1},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Ranges)
		preright := proto.Clone(tc.right).(*mesos.Value_Ranges)
		x := tc.left.Compare(tc.right)
		if x != tc.want {
			t.Errorf("test case %d failed: expected %#+v instead of %#+v", i, tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Ranges_Add(t *testing.T) {
	for i, tc := range []struct {
		left, right, want *mesos.Value_Ranges
	}{
		{nil, nil, ranges()},
		{nil, ranges(r(0, 0)), ranges(r(0, 0))},
		{ranges(r(0, 0)), nil, ranges(r(0, 0))},
		{ranges(r(0, 0)), ranges(r(0, 0)), ranges(r(0, 0))},
		{ranges(r(0, 1)), ranges(r(0, 0)), ranges(r(0, 1))},
		{ranges(r(0, 0)), ranges(r(0, 1)), ranges(r(0, 1))},
		{ranges(r(0, 1)), ranges(r(1, 1)), ranges(r(0, 1))},
		{ranges(r(0, 1)), ranges(r(1, 2)), ranges(r(0, 2))},
		{ranges(r(3, 4), r(0, 1)), ranges(r(1, 2)), ranges(r(0, 4))},
		{ranges(r(2, 6), r(3, 4), r(0, 1)), ranges(r(1, 2)), ranges(r(0, 6))},
		{ranges(r(1, 10), r(5, 30), r(50, 60)), ranges(r(1, 65), r(70, 80)), ranges(r(1, 65), r(70, 80))},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Ranges)
		preright := proto.Clone(tc.right).(*mesos.Value_Ranges)
		x := tc.left.Add(tc.right)
		if !reflect.DeepEqual(x, tc.want) {
			t.Errorf("test case %d failed: expected %#+v instead of %#+v", i, tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Ranges_Subtract(t *testing.T) {
	for i, tc := range []struct {
		left, right, want *mesos.Value_Ranges
	}{
		{nil, nil, ranges()},
		{nil, ranges(r(0, 0)), ranges()},
		{ranges(r(0, 0)), nil, ranges(r(0, 0))},
		{ranges(r(0, 0)), ranges(r(0, 0)), ranges()},
		{ranges(r(0, 1)), ranges(r(0, 0)), ranges(r(1, 1))},
		{ranges(r(0, 0)), ranges(r(0, 1)), ranges()},
		{ranges(r(0, 1)), ranges(r(1, 1)), ranges(r(0, 0))},
		{ranges(r(0, 1)), ranges(r(1, 2)), ranges(r(0, 0))},
		{ranges(r(3, 4), r(0, 1)), ranges(r(1, 2)), ranges(r(0, 0), r(3, 4))},
		{ranges(r(2, 6), r(3, 4), r(0, 1)), ranges(r(2, 4)), ranges(r(0, 1), r(5, 6))},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Ranges)
		preright := proto.Clone(tc.right).(*mesos.Value_Ranges)
		x := tc.left.Subtract(tc.right)
		if !reflect.DeepEqual(x, tc.want) {
			t.Errorf("test case %d failed: expected %#+v instead of %#+v", i, tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Scalar_Add(t *testing.T) {
	for i, tc := range []struct {
		left, right, want *mesos.Value_Scalar
	}{
		{nil, nil, scalar(0)},
		{nil, scalar(0), scalar(0)},
		{scalar(0), nil, scalar(0)},
		{scalar(0), scalar(0), scalar(0)},
		{scalar(1), scalar(0), scalar(1)},
		{scalar(0), scalar(1), scalar(1)},
		{scalar(-1), scalar(0), scalar(-1)},
		{scalar(1), scalar(-1), scalar(0)},
		{scalar(1), scalar(1), scalar(2)},
		{scalar(-1), scalar(-1), scalar(-2)},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Scalar)
		preright := proto.Clone(tc.right).(*mesos.Value_Scalar)
		x := tc.left.Add(tc.right)
		if !x.Equal(tc.want) {
			t.Errorf("expected %v instead of %v", tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Scalar_Subtract(t *testing.T) {
	for i, tc := range []struct {
		left, right, want *mesos.Value_Scalar
	}{
		{nil, nil, scalar(0)},
		{nil, scalar(0), scalar(0)},
		{scalar(0), nil, scalar(0)},
		{scalar(0), scalar(0), scalar(0)},
		{scalar(1), scalar(0), scalar(1)},
		{scalar(0), scalar(1), scalar(-1)},
		{scalar(-1), scalar(0), scalar(-1)},
		{scalar(1), scalar(-1), scalar(2)},
		{scalar(1), scalar(1), scalar(0)},
		{scalar(-1), scalar(-1), scalar(0)},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Scalar)
		preright := proto.Clone(tc.right).(*mesos.Value_Scalar)
		x := tc.left.Subtract(tc.right)
		if !x.Equal(tc.want) {
			t.Errorf("expected %v instead of %v", tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}

func TestValue_Scalar_Compare(t *testing.T) {
	for i, tc := range []struct {
		left, right *mesos.Value_Scalar
		want        int
	}{
		{nil, nil, 0},
		{nil, scalar(0), 0},
		{scalar(0), nil, 0},
		{scalar(0), scalar(0), 0},
		{scalar(1), scalar(0), 1},
		{scalar(0), scalar(1), -1},
		{scalar(-1), scalar(0), -1},
		{scalar(1), scalar(-1), 1},
		{scalar(1), scalar(1), 0},
		{scalar(-1), scalar(-1), 0},
	} {
		preleft := proto.Clone(tc.left).(*mesos.Value_Scalar)
		preright := proto.Clone(tc.right).(*mesos.Value_Scalar)
		x := tc.left.Compare(tc.right)
		if x != tc.want {
			t.Errorf("test case %d failed: expected %v instead of %v", i, tc.want, x)
		}
		if !preleft.Equal(tc.left) {
			t.Errorf("test case %d failed: before(left) != after(left): %#+v != %#+v", i, preleft, tc.left)
		}
		if !preright.Equal(tc.right) {
			t.Errorf("test case %d failed: before(right) != after(right): %#+v != %#+v", i, preright, tc.right)
		}
	}
}
