package eventrules

// go generate -import github.com/mesos/mesos-go/api/v1/lib/scheduler -type E:*scheduler.Event:&scheduler.Event{}
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

func prototype() *scheduler.Event { return &scheduler.Event{} }

func counter(i *int) Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, ch Chain) (context.Context, *scheduler.Event, error) {
		*i++
		return ch(ctx, e, err)
	}
}

func tracer(r Rule, name string, t *testing.T) Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, ch Chain) (context.Context, *scheduler.Event, error) {
		t.Log("executing", name)
		return r(ctx, e, err, ch)
	}
}

func returnError(re error) Rule {
	return func(ctx context.Context, e *scheduler.Event, err error, ch Chain) (context.Context, *scheduler.Event, error) {
		return ch(ctx, e, Error2(err, re))
	}
}

func chainCounter(i *int, ch Chain) Chain {
	return func(ctx context.Context, e *scheduler.Event, err error) (context.Context, *scheduler.Event, error) {
		*i++
		return ch(ctx, e, err)
	}
}

func chainPanic(x interface{}) Chain {
	return func(_ context.Context, _ *scheduler.Event, _ error) (context.Context, *scheduler.Event, error) {
		panic(x)
	}
}

func TestChainIdentity(t *testing.T) {
	var i int
	counterRule := counter(&i)

	_, e, err := Rules{counterRule}.Eval(context.Background(), nil, nil, ChainIdentity)
	if e != nil {
		t.Error("expected nil event instead of", e)
	}
	if err != nil {
		t.Error("expected nil error instead of", err)
	}
	if i != 1 {
		t.Error("expected 1 rule execution instead of", i)
	}
}

func TestRules(t *testing.T) {
	var (
		p   = prototype()
		a   = errors.New("a")
		ctx = context.Background()
	)

	// multiple rules in Rules should execute, dropping nil rules along the way
	for _, tc := range []struct {
		e   *scheduler.Event
		err error
	}{
		{nil, nil},
		{nil, a},
		{p, nil},
		{p, a},
	} {
		var (
			i    int
			rule = New(
				nil,
				tracer(counter(&i), "counter1", t),
				nil,
				tracer(counter(&i), "counter2", t),
				nil,
			)
			_, e, err = rule(ctx, tc.e, tc.err, ChainIdentity)
		)
		if e != tc.e {
			t.Errorf("expected prototype event %q instead of %q", tc.e, e)
		}
		if err != tc.err {
			t.Errorf("expected %q error instead of %q", tc.err, err)
		}
		if i != 2 {
			t.Error("expected 2 rule executions instead of", i)
		}

		// empty Rules should not change event, err
		_, e, err = Rules{}.Eval(ctx, tc.e, tc.err, ChainIdentity)
		if e != tc.e {
			t.Errorf("expected prototype event %q instead of %q", tc.e, e)
		}
		if err != tc.err {
			t.Errorf("expected %q error instead of %q", tc.err, err)
		}
	}
}

func TestError(t *testing.T) {
	a := errors.New("a")
	list := ErrorList{a}

	msg := list.Error()
	if msg != a.Error() {
		t.Errorf("expected %q instead of %q", a.Error(), msg)
	}

	msg = ErrorList{}.Error()
	if msg != msgNoErrors {
		t.Errorf("expected %q instead of %q", msgNoErrors, msg)
	}

	msg = ErrorList(nil).Error()
	if msg != msgNoErrors {
		t.Errorf("expected %q instead of %q", msgNoErrors, msg)
	}
}

func TestError2(t *testing.T) {
	var (
		a = errors.New("a")
		b = errors.New("b")
	)
	for i, tc := range []struct {
		a            error
		b            error
		wants        error
		wantsMessage string
	}{
		{nil, nil, nil, ""},
		{nil, ErrorList{nil}, nil, ""},
		{ErrorList{nil}, ErrorList{nil}, nil, ""},
		{ErrorList{ErrorList{nil}}, ErrorList{nil}, nil, ""},
		{a, nil, a, "a"},
		{ErrorList{a}, nil, a, "a"},
		{ErrorList{nil, a, ErrorList{}}, nil, a, "a"},
		{nil, b, b, "b"},
		{nil, ErrorList{b}, b, "b"},
		{a, b, ErrorList{a, b}, "a (and 1 more errors)"},
		{a, ErrorList{b}, ErrorList{a, b}, "a (and 1 more errors)"},
		{a, ErrorList{nil, ErrorList{b, ErrorList{}, nil}}, ErrorList{a, b}, "a (and 1 more errors)"},
	} {
		var (
			sameError bool
			result    = Error2(tc.a, tc.b)
		)
		// jump through hoops because we can't directly compare two errors with == if
		// they're both ErrorList.
		if IsErrorList(result) == IsErrorList(tc.wants) { // both are lists or neither
			sameError = (!IsErrorList(result) && result == tc.wants) ||
				(IsErrorList(result) && reflect.DeepEqual(result, tc.wants))
		}
		if !sameError {
			t.Fatalf("test case %d failed, expected %v instead of %v", i, tc.wants, result)
		}
		if result != nil && tc.wantsMessage != result.Error() {
			t.Fatalf("test case %d failed, expected message %q instead of %q",
				i, tc.wantsMessage, result.Error())
		}
	}
}

func TestUnlessDone(t *testing.T) {
	var (
		p   = prototype()
		ctx = context.Background()
		fin = func() context.Context {
			c, cancel := context.WithCancel(context.Background())
			cancel()
			return c
		}()
	)
	for ti, tc := range []struct {
		ctx             context.Context
		wantsError      []error
		wantsRuleCount  []int
		wantsChainCount []int
	}{
		{ctx, []error{nil, nil}, []int{1, 2}, []int{1, 2}},
		{fin, []error{nil, context.Canceled}, []int{1, 1}, []int{1, 1}},
	} {
		var (
			i, j int
			r1   = counter(&i)
			r2   = r1.UnlessDone()
		)
		for k, r := range []Rule{r1, r2} {
			_, e, err := r(tc.ctx, p, nil, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("test case %d failed: expected event %q instead of %q", ti, p, e)
			}
			if err != tc.wantsError[k] {
				t.Errorf("test case %d failed: unexpected error %v", ti, err)
			}
			if i != tc.wantsRuleCount[k] {
				t.Errorf("test case %d failed: expected count of %d instead of %d", ti, tc.wantsRuleCount[k], i)
			}
			if j != tc.wantsChainCount[k] {
				t.Errorf("test case %d failed: expected chain count of %d instead of %d", ti, tc.wantsRuleCount[k], j)
			}
		}
	}
	r := Rule(nil).UnlessDone()
	if r != nil {
		t.Error("expected nil result from UnlessDone")
	}
}

func TestAndThen(t *testing.T) {
	var (
		i, j int
		p    = prototype()
		ctx  = context.Background()
		r1   = counter(&i)
		r2   = Rule(nil).AndThen(counter(&i))
		a    = errors.New("a")
	)
	for k, r := range []Rule{r1, r2} {
		_, e, err := r(ctx, p, a, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		if err != a {
			t.Error("unexpected error", err)
		}
		if i != 1 {
			t.Errorf("expected count of 1 instead of %d", i)
		}
		if j != (k + 1) {
			t.Errorf("expected chain count of %d instead of %d", (k + 1), j)
		}
	}
}

func TestOnFailure(t *testing.T) {
	var (
		i, j int
		p    = prototype()
		ctx  = context.Background()
		a    = errors.New("a")
		r1   = counter(&i)
		r2   = Fail(a).OnFailure(counter(&i))
	)
	for k, tc := range []struct {
		r            Rule
		initialError error
	}{
		{r1, a},
		{r2, nil},
	} {
		_, e, err := tc.r(ctx, p, tc.initialError, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		if err != a {
			t.Error("unexpected error", err)
		}
		if i != (k + 1) {
			t.Errorf("expected count of %d instead of %d", (k + 1), i)
		}
		if j != (k + 1) {
			t.Errorf("expected chain count of %d instead of %d", (k + 1), j)
		}
	}
}

func TestDropOnError(t *testing.T) {
	var (
		i, j int
		p    = prototype()
		ctx  = context.Background()
		r1   = counter(&i)
		r2   = counter(&i).DropOnError()
		a    = errors.New("a")
	)
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for _, r := range []Rule{r1, r2} {
		_, e, err := r(ctx, p, a, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		if err != a {
			t.Error("unexpected error", err)
		}
		if i != 1 {
			t.Errorf("expected count of 1 instead of %d", i)
		}
		if j != 1 {
			t.Errorf("expected chain count of 1 instead of %d", j)
		}
	}
	_, e, err := r2(ctx, p, nil, chainCounter(&j, ChainIdentity))
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	if err != nil {
		t.Error("unexpected error", err)
	}
	if j != 2 {
		t.Errorf("expected chain count of 2 instead of %d", j)
	}
}

func TestDropOnSuccess(t *testing.T) {
	var (
		i, j int
		p    = prototype()
		ctx  = context.Background()
		r1   = counter(&i)
		r2   = counter(&i).DropOnSuccess()
	)
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for _, r := range []Rule{r1, r2} {
		_, e, err := r(ctx, p, nil, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		if err != nil {
			t.Error("unexpected error", err)
		}
		if i != 1 {
			t.Errorf("expected count of 1 instead of %d", i)
		}
		if j != 1 {
			t.Errorf("expected chain count of 1 instead of %d", j)
		}
	}
	a := errors.New("a")
	_, e, err := r2(ctx, p, a, chainCounter(&j, ChainIdentity))
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	if err != a {
		t.Error("unexpected error", err)
	}
	if i != 2 {
		t.Errorf("expected count of 2 instead of %d", i)
	}
	if j != 2 {
		t.Errorf("expected chain count of 2 instead of %d", j)
	}

	r3 := Rules{DropOnSuccess(), r1}.Eval
	_, e, err = r3(ctx, p, nil, chainCounter(&j, ChainIdentity))
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	if err != nil {
		t.Error("unexpected error", err)
	}
	if i != 2 {
		t.Errorf("expected count of 2 instead of %d", i)
	}
	if j != 3 {
		t.Errorf("expected chain count of 3 instead of %d", j)
	}
}

func TestThenDrop(t *testing.T) {
	for _, anErr := range []error{nil, errors.New("a")} {
		var (
			i, j int
			p    = prototype()
			ctx  = context.Background()
			r1   = counter(&i)
			r2   = counter(&i).ThenDrop()
		)
		// r1 and r2 should execute the counter rule
		for k, r := range []Rule{r1, r2} {
			_, e, err := r(ctx, p, anErr, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("expected event %q instead of %q", p, e)
			}
			if err != anErr {
				t.Errorf("expected %v instead of error %v", anErr, err)
			}
			if i != (k + 1) {
				t.Errorf("expected count of %d instead of %d", (k + 1), i)
			}
			if j != 1 {
				t.Errorf("expected chain count of 1 instead of %d", j)
			}
		}
	}
}

func TestDrop(t *testing.T) {
	for _, anErr := range []error{nil, errors.New("a")} {
		var (
			i, j int
			p    = prototype()
			ctx  = context.Background()
			r1   = counter(&i)
			r2   = Rules{Drop(), counter(&i)}.Eval
		)
		// r1 should execute the counter rule
		// r2 should NOT exexute the counter rule
		for k, r := range []Rule{r1, r2} {
			_, e, err := r(ctx, p, anErr, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("expected event %q instead of %q", p, e)
			}
			if err != anErr {
				t.Errorf("expected %v instead of error %v", anErr, err)
			}
			if i != 1 {
				t.Errorf("expected count of 1 instead of %d", i)
			}
			if j != (k + 1) {
				t.Errorf("expected chain count of %d instead of %d with error %v", (k + 1), j, anErr)
			}
		}
	}
}

func TestIf(t *testing.T) {
	var (
		i, j int
		p    = prototype()
		ctx  = context.Background()
		r1   = counter(&i).If(true).Eval
		r2   = counter(&i).If(false).Eval
	)
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for k, r := range []Rule{r1, r2} {
		_, e, err := r(ctx, p, nil, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		if err != nil {
			t.Error("unexpected error", err)
		}
		if i != 1 {
			t.Errorf("expected count of 1 instead of %d", i)
		}
		if j != (k + 1) {
			t.Errorf("expected chain count of %d instead of %d", (k + 1), j)
		}
	}
}

func TestUnless(t *testing.T) {
	var (
		i, j int
		p    = prototype()
		ctx  = context.Background()
		r1   = counter(&i).Unless(false).Eval
		r2   = counter(&i).Unless(true).Eval
	)
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for k, r := range []Rule{r1, r2} {
		_, e, err := r(ctx, p, nil, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		if err != nil {
			t.Error("unexpected error", err)
		}
		if i != 1 {
			t.Errorf("expected count of 1 instead of %d", i)
		}
		if j != (k + 1) {
			t.Errorf("expected chain count of %d instead of %d", (k + 1), j)
		}
	}
}

func TestOnce(t *testing.T) {
	var (
		i, j int
		p    = prototype()
		ctx  = context.Background()
		r1   = counter(&i).Once().Eval
		r2   = Rule(nil).Once().Eval
	)
	for k, r := range []Rule{r1, r2} {
		for x := 0; x < 5; x++ {
			_, e, err := r(ctx, p, nil, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("expected event %q instead of %q", p, e)
			}
			if err != nil {
				t.Error("unexpected error", err)
			}
			if i != 1 {
				t.Errorf("expected count of 1 instead of %d", i)
			}
			if y := (k * 5) + x + 1; j != y {
				t.Errorf("expected chain count of %d instead of %d", y, j)
			}
		}
	}
}

func TestRateLimit(t *testing.T) {
	// non-blocking, then blocking
	o := func() <-chan struct{} {
		x := make(chan struct{}, 1)
		x <- struct{}{}
		return x
	}
	var (
		ch1 <-chan struct{}       // always nil, blocking
		ch2 = make(chan struct{}) // non-nil, blocking
		// ch3 is o()
		ch4 = make(chan struct{}) // non-nil, closed
		p   = prototype()
		ctx = context.Background()
		fin = func() context.Context {
			c, cancel := context.WithCancel(context.Background())
			cancel()
			return c
		}()

		errOverflow               = errors.New("overflow")
		otherwiseSkip             = Rule(nil)
		otherwiseSkipWithError    = Fail(errOverflow)
		otherwiseDiscard          = Drop()
		otherwiseDiscardWithError = Fail(errOverflow).ThenDrop()
	)
	close(ch4)
	for ti, tc := range []struct {
		// each set of inputs is executed 4 times: twice for r1, twice for r2
		ctx             context.Context
		ch              <-chan struct{}
		over            Overflow
		otherwise       Rule
		wantsError      int // bitmask: lower 4 bits, one for each case; first case = highest bit
		wantsRuleCount  []int
		wantsChainCount []int
	}{
		{ctx, ch1, OverflowOtherwise, otherwiseSkip, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{ctx, ch2, OverflowOtherwise, otherwiseSkip, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{ctx, o(), OverflowOtherwise, otherwiseSkip, 0x0, []int{1, 1, 1, 1}, []int{1, 2, 3, 4}},
		{ctx, ch4, OverflowOtherwise, otherwiseSkip, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{fin, ch1, OverflowOtherwise, otherwiseSkip, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{fin, ch2, OverflowOtherwise, otherwiseSkip, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{fin, o(), OverflowOtherwise, otherwiseSkip, 0x0, []int{1, 1, 1, 1}, []int{1, 2, 3, 4}},
		{fin, ch4, OverflowOtherwise, otherwiseSkip, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{ctx, ch1, OverflowOtherwise, otherwiseSkipWithError, 0xC, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{ctx, ch2, OverflowOtherwise, otherwiseSkipWithError, 0xC, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{ctx, o(), OverflowOtherwise, otherwiseSkipWithError, 0x4, []int{1, 1, 1, 1}, []int{1, 2, 3, 4}},
		{ctx, ch4, OverflowOtherwise, otherwiseSkipWithError, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{fin, ch1, OverflowOtherwise, otherwiseSkipWithError, 0xC, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{fin, ch2, OverflowOtherwise, otherwiseSkipWithError, 0xC, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{fin, o(), OverflowOtherwise, otherwiseSkipWithError, 0x4, []int{1, 1, 1, 1}, []int{1, 2, 3, 4}},
		{fin, ch4, OverflowOtherwise, otherwiseSkipWithError, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{ctx, ch1, OverflowOtherwise, otherwiseDiscard, 0x0, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{ctx, ch2, OverflowOtherwise, otherwiseDiscard, 0x0, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{ctx, o(), OverflowOtherwise, otherwiseDiscard, 0x0, []int{1, 1, 1, 1}, []int{1, 1, 2, 3}},
		{ctx, ch4, OverflowOtherwise, otherwiseDiscard, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{fin, ch1, OverflowOtherwise, otherwiseDiscard, 0x0, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{fin, ch2, OverflowOtherwise, otherwiseDiscard, 0x0, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{fin, o(), OverflowOtherwise, otherwiseDiscard, 0x0, []int{1, 1, 1, 1}, []int{1, 1, 2, 3}},
		{fin, ch4, OverflowOtherwise, otherwiseDiscard, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{ctx, ch1, OverflowOtherwise, otherwiseDiscardWithError, 0xC, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{ctx, ch2, OverflowOtherwise, otherwiseDiscardWithError, 0xC, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{ctx, o(), OverflowOtherwise, otherwiseDiscardWithError, 0x4, []int{1, 1, 1, 1}, []int{1, 1, 2, 3}},
		{ctx, ch4, OverflowOtherwise, otherwiseDiscardWithError, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{fin, ch1, OverflowOtherwise, otherwiseDiscardWithError, 0xC, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{fin, ch2, OverflowOtherwise, otherwiseDiscardWithError, 0xC, []int{0, 0, 0, 0}, []int{0, 0, 1, 2}},
		{fin, o(), OverflowOtherwise, otherwiseDiscardWithError, 0x4, []int{1, 1, 1, 1}, []int{1, 1, 2, 3}},
		{fin, ch4, OverflowOtherwise, otherwiseDiscardWithError, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},

		{fin, ch1, OverflowWait, nil, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{fin, ch2, OverflowWait, nil, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{fin, o(), OverflowWait, nil, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{fin, ch4, OverflowWait, nil, 0x0, []int{0, 0, 0, 0}, []int{1, 2, 3, 4}},
		{ctx, ch4, OverflowWait, nil, 0x0, []int{1, 2, 2, 2}, []int{1, 2, 3, 4}},
	} {
		var (
			i, j int
			r1   = counter(&i).RateLimit(tc.ch, tc.over, tc.otherwise).Eval
			r2   = Rule(nil).RateLimit(tc.ch, tc.over, tc.otherwise).Eval // a nil rule still invokes the chain
		)
		for k, r := range []Rule{r1, r2} {
			// execute each rule twice
			for x := 0; x < 2; x++ {
				_, e, err := r(tc.ctx, p, nil, chainCounter(&j, ChainIdentity))
				if e != p {
					t.Errorf("test case %d failed: expected event %q instead of %q", ti, p, e)
				}
				if b := 8 >> uint(k*2+x); ((b & tc.wantsError) != 0) != (err != nil) {
					t.Errorf("test case (%d,%d,%d) failed: unexpected error %v", ti, k, x, err)
				}
				if y := tc.wantsRuleCount[k*2+x]; i != y {
					t.Errorf("test case (%d,%d,%d) failed: expected count of %d instead of %d",
						ti, k, x, y, i)
				}
				if y := tc.wantsChainCount[k*2+x]; j != y {
					t.Errorf("test case (%d,%d,%d) failed: expected chain count of %d instead of %d",
						ti, k, x, y, j)
				}
			}
		}
	}
	// test blocking capability via rateLimit
	blocked := false
	r := limit(Rule(nil).Eval, func(_ context.Context, b bool) bool { blocked = b; return false }, OverflowWait, nil)
	_, e, err := r(ctx, p, nil, ChainIdentity)
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if !blocked {
		t.Error("expected OverflowWait to block rule execution")
	}
	// test RateLimit deadlock detector
	didPanic := false
	func() {
		defer func() { didPanic = recover() != nil }()
		Rule(Rule(nil).Eval).RateLimit(nil, OverflowWait, nil).Eval(ctx, p, nil, ChainIdentity)
	}()
	if !didPanic {
		t.Error("expected panic because we configured a rule to deadlock")
	}
}
