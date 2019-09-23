// +build ignore

package main

import (
	"os"
	"text/template"
)

func main() {
	Run(rulesTemplate, rulesTestTemplate, os.Args...)
}

var rulesTemplate = template.Must(template.New("").Parse(`package {{.Package}}

// go generate {{.Args}}
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"fmt"
	"sync"
{{range .Imports}}
	{{ printf "%q" . -}}
{{end}}
)

{{.RequireType "E" -}}
{{.RequirePrototype "E" -}}
{{.RequirePrototype "Z" -}}{{/* Z is an optional type, but if it's specified then it needs a prototype */ -}}
type (
	evaler interface {
		// Eval executes a filter, rule, or decorator function; if the returned event is nil then
		// no additional rule evaluation should be processed for the event.
		// Eval implementations should not modify the given event parameter (to avoid side effects).
		// If changes to the event object are needed, the suggested approach is to make a copy,
		// modify the copy, and pass the copy to the chain.
		// Eval implementations SHOULD be safe to execute concurrently.
		Eval(context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error, Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error)
	}

	// Rule is the functional adaptation of evaler.
	// A nil Rule is valid: it is Eval'd as a noop.
	Rule func(context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error, Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error)

	// Chain is invoked by a Rule to continue processing an event. If the chain is not invoked,
	// no additional rules are processed.
	Chain func(context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error)

	// Rules is a list of rules to be processed, in order.
	Rules []Rule

	// ErrorList accumulates errors that occur while processing a Chain of Rules. Accumulated
	// errors should be appended to the end of the list. An error list should never be empty.
	// Callers should use the package Error() func to properly accumulate (and flatten) errors.
	ErrorList []error
)

var (
	_ = evaler(Rule(nil))
	_ = evaler(Rules{})
)

// ChainIdentity is a Chain that returns the arguments as its results.
func ChainIdentity(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
	return ctx, e, {{.Ref "Z" "z," -}} err
}

// Eval is a convenience func that processes a nil Rule as a noop.
func (r Rule) Eval(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
	if r != nil {
		return r(ctx, e, {{.Ref "Z" "z," -}} err, ch)
	}
	return ch(ctx, e, {{.Ref "Z" "z," -}} err)
}

// Eval is a Rule func that processes the set of all Rules. If there are no rules in the
// set then control is simply passed to the Chain.
func (rs Rules) Eval(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
	return ch(rs.Chain()(ctx, e, {{.Ref "Z" "z," -}} err))
}

// Chain returns a Chain that evaluates the given Rules, in order, propagating the (context.Context, {{.Type "E"}}, error)
// from Rule to Rule. Chain is safe to invoke concurrently.
func (rs Rules) Chain() Chain {
	if len(rs) == 0 {
		return ChainIdentity
	}
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		return rs[0].Eval(ctx, e, {{.Ref "Z" "z," -}} err, rs[1:].Chain())
	}
}

// New is the semantic equivalent of Rules{r1, r2, ..., rn}.Eval and exists purely for convenience.
func New(rs ...Rule) Rule { return Rules(rs).Eval }

const msgNoErrors = "no errors"

// Error implements error; returns the message of the first error in the list.
func (es ErrorList) Error() string {
	switch len(es) {
	case 0:
		return msgNoErrors
	case 1:
		return es[0].Error()
	default:
		return fmt.Sprintf("%s (and %d more errors)", es[0], len(es)-1)
	}
}

// Error2 aggregates the given error params, returning nil if both are nil.
// Use Error2 to avoid the overhead of creating a slice when aggregating only 2 errors.
func Error2(a, b error) error {
	if a == nil {
		if b == nil {
			return nil
		}
		if list, ok := b.(ErrorList); ok {
			return flatten(list).Err()
		}
		return b
	}
	if b == nil {
		if list, ok := a.(ErrorList); ok {
			return flatten(list).Err()
		}
		return a
	}
	return Error(a, b)
}

// Err reduces an empty or singleton error list
func (es ErrorList) Err() error {
	switch len(es) {
	case 0:
		return nil
	case 1:
		return es[0]
	default:
		return es
	}
}

// IsErrorList returns true if err is a non-nil error list
func IsErrorList(err error) bool {
	if err != nil {
		_, ok := err.(ErrorList)
		return ok
	}
	return false
}

// Error aggregates, and then flattens, a list of errors accrued during rule processing.
// Returns nil if the given list of errors is empty or contains all nil errors.
func Error(es ...error) error {
	return flatten(es).Err()
}

func flatten(errors []error) ErrorList {
	if errors == nil || len(errors) == 0 {
		return nil
	}
	result := make([]error, 0, len(errors))
	for _, err := range errors {
		if err != nil {
			if multi, ok := err.(ErrorList); ok {
				result = append(result, flatten(multi)...)
			} else {
				result = append(result, err)
			}
		}
	}
	return ErrorList(result)
}

// TODO(jdef): other ideas for Rule decorators: When(func() bool), WhenNot(func() bool)

// If only executes the receiving rule if b is true; otherwise, the returned rule is a noop.
func (r Rule) If(b bool) Rule {
	if b {
		return r
	}
	return nil
}

// Unless only executes the receiving rule if b is false; otherwise, the returned rule is a noop.
func (r Rule) Unless(b bool) Rule {
	if !b {
		return r
	}
	return nil
}

// Once returns a Rule that executes the receiver only once.
func (r Rule) Once() Rule {
	if r == nil {
		return nil
	}
	var once sync.Once
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		ruleInvoked := false
		once.Do(func() {
			ctx, e, {{.Ref "Z" "z," -}} err = r(ctx, e, {{.Ref "Z" "z," -}} err, ch)
			ruleInvoked = true
		})
		if !ruleInvoked {
			ctx, e, {{.Ref "Z" "z," -}} err = ch(ctx, e, {{.Ref "Z" "z," -}} err)
		}
		return ctx, e, {{.Ref "Z" "z," -}} err
	}
}

// UnlessDone returns a decorated rule that checks context.Done: if the context has been canceled then the rule chain
// is aborted and the context.Err is merged with the current error state.
// Returns nil (noop) if the receiving Rule is nil.
func (r Rule) UnlessDone() Rule {
	if r == nil {
		return nil
	}
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		select {
		case <-ctx.Done():
			return ctx, e, {{.Ref "Z" "z," -}} Error2(err, ctx.Err())
		default:
			return r(ctx, e, {{.Ref "Z" "z," -}} err, ch)
		}
	}
}

type Overflow int

const (
	// OverflowWait waits until the rule may execute, or the context is canceled.
	OverflowWait Overflow = iota
	// OverflowOtherwise skips over the decorated rule and invoke an alternative instead.
	OverflowOtherwise
)

// RateLimit invokes the receiving Rule if a read of chan "p" succeeds (closed chan = no rate limit), otherwise proceeds
// according to the specified Overflow policy. May be useful, for example, when rate-limiting logged events.
// Returns nil (noop) if the receiver is nil, otherwise a nil chan will normally trigger an overflow.
// Panics when OverflowWait is specified with a nil chan, in order to prevent deadlock.
// A cancelled context will trigger the "otherwise" rule.
func (r Rule) RateLimit(p <-chan struct{}, over Overflow, otherwise Rule) Rule {
	return limit(r, acquireChan(p), over, otherwise)
}

// acquireChan wraps a signal chan with a func that can be used with rateLimit.
// should only be called by rate limiting funcs (that implement deadlock avoidance).
func acquireChan(tokenCh <-chan struct{}) func(context.Context, bool) bool {
	if tokenCh == nil {
		// always false: acquire never succeeds; panic if told to block (to avoid deadlock)
		return func(ctx context.Context, block bool) bool {
			if block {
				select {
				case <-ctx.Done():
				default:
					panic("deadlock detected: block should never be true when the token chan is nil")
				}
			}
			return false
		}
	}
	return func(ctx context.Context, block bool) bool {
		if block {
			select {
			case <-tokenCh:
				// tie breaker prefers Done
				select {
				case <-ctx.Done():
				default:
					return true
				}
			case <-ctx.Done():
			}
			return false
		}
		select {
		case <-tokenCh:
			return true
		default:
			return false
		}
	}
}

// limit is a generic Rule decorator that limits invocations of said Rule.
// The "acquire" func SHOULD NOT block if the supplied Context is Done.
// MUST only invoke "acquire" once per event.
// TODO(jdef): leaving this as internal for now because the interface still feels too messy.
func limit(r Rule, acquire func(_ context.Context, block bool) bool, over Overflow, otherwise Rule) Rule {
	if r == nil {
		return nil
	}
	if acquire == nil {
		panic("acquire func is not allowed to be nil")
	}
	blocking := false
	switch over {
	case OverflowOtherwise:
	case OverflowWait:
		blocking = true
	default:
		panic(fmt.Sprintf("unexpected Overflow type: %#v", over))
	}
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		if !acquire(ctx, blocking) {
			return otherwise.Eval(ctx, e, {{.Ref "Z" "z," -}} err, ch)
		}
		return r(ctx, e, {{.Ref "Z" "z," -}} err, ch)
	}
}

/* TODO(jdef) not sure that this is very useful, leaving out for now...

// EveryN invokes the receiving rule beginning with the first event seen and then every n'th
// time after that. If nthTime is less then 2 then the receiver is returned, undecorated.
// The "otherwise" Rule (may be null) is invoked for every event in between the n'th invocations.
// A cancelled context will trigger the "otherwise" rule.
func (r Rule) EveryN(nthTime int, otherwise Rule) Rule {
	if nthTime < 2 || r == nil {
		return r
	}
	return limit(r, acquireEveryN(nthTime), OverflowOtherwise, otherwise)
}

// acquireEveryN returns an "acquire" func (for use w/ rate-limiting) that returns true every N'th invocation.
// the returned func MUST NOT be used with a potentially blocking Overflow policy (or else it panics).
// nthTime SHOULD be greater than math.MinInt32, values less than 2 probably don't make sense in practice.
func acquireEveryN(nthTime int) func(context.Context, bool) bool {
	var (
		i       = 1 // begin with the first event seen
		m       sync.Mutex
	)
	return func(ctx context.Context, block bool) (result bool) {
		if block {
			panic("acquireEveryN should never be asked to block")
		}
		select {
		case <-ctx.Done():
		default:
			m.Lock()
			i--
			if i <= 0 {
				i = nthTime
				result = true
			}
			m.Unlock()
		}
		return
	}
}

*/

// Drop aborts the Chain and returns the (context.Context, {{.Type "E"}}, error) tuple as-is.
func Drop() Rule {
	return Rule(nil).ThenDrop()
}

// ThenDrop executes the receiving rule, but aborts the Chain, and returns the (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) tuple as-is.
func (r Rule) ThenDrop() Rule {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, _ Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		return r.Eval(ctx, e, {{.Ref "Z" "z," -}} err, ChainIdentity)
	}
}

// Fail returns a Rule that injects the given error.
func Fail(injected error) Rule {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		return ch(ctx, e, {{.Ref "Z" "z," -}} Error2(err, injected))
	}
}

// DropOnError returns a Rule that generates a nil event if the error state != nil
func DropOnError() Rule {
	return Rule(nil).DropOnError()
}

// DropOnError decorates a rule by pre-checking the error state: if the error state != nil then
// the receiver is not invoked and (e, err) is returned; otherwise control passes to the receiving rule.
func (r Rule) DropOnError() Rule {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		if err != nil {
			return ctx, e, {{.Ref "Z" "z," -}} err
		}
		return r.Eval(ctx, e, {{.Ref "Z" "z," -}} err, ch)
	}
}

// AndThen returns a list of rules, beginning with the receiver, followed by DropOnError, and then
// all of the rules specified by the next parameter. The net effect is: execute the receiver rule
// and only if there is no error state, continue processing the next rules, in order.
func (r Rule) AndThen(next ...Rule) Rule {
	return append(Rules{r, DropOnError()}, next...).Eval
}

func DropOnSuccess() Rule {
	return Rule(nil).DropOnSuccess()
}

func (r Rule) DropOnSuccess() Rule {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		if err == nil {
			// bypass remainder of chain
			return ctx, e, {{.Ref "Z" "z," -}} err
		}
		return r.Eval(ctx, e, {{.Ref "Z" "z," -}} err, ch)
	}
}

func (r Rule) OnFailure(next ...Rule) Rule {
	return append(Rules{r, DropOnSuccess()}, next...).Eval
}
`))

var rulesTestTemplate = template.Must(template.New("").Parse(`package {{.Package}}

// go generate {{.Args}}
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"errors"
	"reflect"
	"testing"
{{range .Imports}}
	{{ printf "%q" . -}}
{{end}}
)

func prototype() {{.Type "E"}} { return {{.Prototype "E"}} }

func counter(i *int) Rule {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		*i++
		return ch(ctx, e, {{.Ref "Z" "z," -}} err)
	}
}

func tracer(r Rule, name string, t *testing.T) Rule {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		t.Log("executing", name)
		return r(ctx, e, {{.Ref "Z" "z," -}} err, ch)
	}
}

func returnError(re error) Rule {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error, ch Chain) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		return ch(ctx, e, {{.Ref "Z" "z," -}} Error2(err, re))
	}
}

func chainCounter(i *int, ch Chain) Chain {
	return func(ctx context.Context, e {{.Type "E"}}, {{.Arg "Z" "z," -}} err error) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		*i++
		return ch(ctx, e, {{.Ref "Z" "z," -}} err)
	}
}

func chainPanic(x interface{}) Chain {
	return func(_ context.Context, _ {{.Type "E"}}, {{.Arg "Z" "_," -}} _ error) (context.Context, {{.Type "E"}}, {{.Arg "Z" "," -}} error) {
		panic(x)
	}
}

func TestChainIdentity(t *testing.T) {
	var i int
	counterRule := counter(&i)
{{if .Type "Z"}}
	{{.Var "Z" "z0"}}
{{end}}
	_, e, {{.Ref "Z" "_," -}} err := Rules{counterRule}.Eval(context.Background(), nil, {{.Ref "Z" "z0," -}} nil, ChainIdentity)
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

	{{if .Type "Z" -}}
	{{.Var "Z" "z0"}}
	var zp = {{.Prototype "Z"}}
	{{end -}}

	// multiple rules in Rules should execute, dropping nil rules along the way
	for _, tc := range []struct {
		e   {{.Type "E"}}
		{{if .Type "Z"}}
		{{- .Arg "Z" "z  "}}
		{{end -}}
		err error
	}{
		{nil, {{.Ref "Z" "z0," -}} nil},
		{nil, {{.Ref "Z" "z0," -}} a},
		{p, {{.Ref "Z" "z0," -}} nil},
		{p, {{.Ref "Z" "z0," -}} a},
{{if .Type "Z"}}
		{nil, {{.Ref "Z" "zp," -}} nil},
		{nil, {{.Ref "Z" "zp," -}} a},
		{p, {{.Ref "Z" "zp," -}} nil},
		{p, {{.Ref "Z" "zp," -}} a},
{{end}}	} {
		var (
			i    int
			rule = New(
				nil,
				tracer(counter(&i), "counter1", t),
				nil,
				tracer(counter(&i), "counter2", t),
				nil,
			)
			_, e, {{.Ref "Z" "zz," -}} err = rule(ctx, tc.e, {{.Ref "Z" "tc.z," -}} tc.err, ChainIdentity)
		)
		if e != tc.e {
			t.Errorf("expected prototype event %q instead of %q", tc.e, e)
		}
		{{if .Type "Z" -}}
		if zz != tc.z {
			t.Errorf("expected return object %q instead of %q", tc.z, zz)
		}
		{{end -}}
		if err != tc.err {
			t.Errorf("expected %q error instead of %q", tc.err, err)
		}
		if i != 2 {
			t.Error("expected 2 rule executions instead of", i)
		}

		// empty Rules should not change event, {{.Ref "Z" "z," -}} err
		_, e, {{.Ref "Z" "zz," -}} err = Rules{}.Eval(ctx, tc.e, {{.Ref "Z" "tc.z," -}} tc.err, ChainIdentity)
		if e != tc.e {
			t.Errorf("expected prototype event %q instead of %q", tc.e, e)
		}
		{{if .Type "Z" -}}
		if zz != tc.z {
			t.Errorf("expected return object %q instead of %q", tc.z, zz)
		}
		{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
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
			_, e, {{.Ref "Z" "zz," -}} err := r(tc.ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("test case %d failed: expected event %q instead of %q", ti, p, e)
			}
			{{if .Type "Z" -}}
			if zz != zp {
				t.Errorf("test case %d failed: expected return object %q instead of %q", ti, zp, zz)
			}
			{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
	for k, r := range []Rule{r1, r2} {
		_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} a, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		{{if .Type "Z" -}}
		if zz != zp {
			t.Errorf("expected return object %q instead of %q", zp, zz)
		}
		{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
	for k, tc := range []struct {
		r            Rule
		initialError error
	}{
		{r1, a},
		{r2, nil},
	} {
		_, e, {{.Ref "Z" "zz," -}} err := tc.r(ctx, p, {{.Ref "Z" "zp," -}} tc.initialError, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		{{if .Type "Z" -}}
		if zz != zp {
			t.Errorf("expected return object %q instead of %q", zp, zz)
		}
		{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for _, r := range []Rule{r1, r2} {
		_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} a, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		{{if .Type "Z" -}}
		if zz != zp {
			t.Errorf("expected return object %q instead of %q", zp, zz)
		}
		{{end -}}
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
	_, e, {{.Ref "Z" "zz," -}} err := r2(ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	{{if .Type "Z" -}}
	if zz != zp {
		t.Errorf("expected return object %q instead of %q", zp, zz)
	}
	{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for _, r := range []Rule{r1, r2} {
		_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		{{if .Type "Z" -}}
		if zz != zp {
			t.Errorf("expected return object %q instead of %q", zp, zz)
		}
		{{end -}}
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
	_, e, {{.Ref "Z" "zz," -}} err := r2(ctx, p, {{.Ref "Z" "zp," -}} a, chainCounter(&j, ChainIdentity))
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	{{if .Type "Z" -}}
	if zz != zp {
		t.Errorf("expected return object %q instead of %q", zp, zz)
	}
	{{end -}}
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
	_, e, {{.Ref "Z" "zz," -}} err = r3(ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	{{if .Type "Z" -}}
	if zz != zp {
		t.Errorf("expected return object %q instead of %q", zp, zz)
	}
	{{end -}}
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
		{{if .Type "Z" -}}
		var zp = {{.Prototype "Z"}}
		{{end -}}
		// r1 and r2 should execute the counter rule
		for k, r := range []Rule{r1, r2} {
			_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} anErr, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("expected event %q instead of %q", p, e)
			}
			{{if .Type "Z" -}}
			if zz != zp {
				t.Errorf("expected return object %q instead of %q", zp, zz)
			}
			{{end -}}
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
		{{if .Type "Z" -}}
		var zp = {{.Prototype "Z"}}
		{{end -}}
		// r1 should execute the counter rule
		// r2 should NOT exexute the counter rule
		for k, r := range []Rule{r1, r2} {
			_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} anErr, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("expected event %q instead of %q", p, e)
			}
			{{if .Type "Z" -}}
			if zz != zp {
				t.Errorf("expected return object %q instead of %q", zp, zz)
			}
			{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for k, r := range []Rule{r1, r2} {
		_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		{{if .Type "Z" -}}
		if zz != zp {
			t.Errorf("expected return object %q instead of %q", zp, zz)
		}
		{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
	// r1 should execute the counter rule
	// r2 should NOT exexute the counter rule
	for k, r := range []Rule{r1, r2} {
		_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
		if e != p {
			t.Errorf("expected event %q instead of %q", p, e)
		}
		{{if .Type "Z" -}}
		if zz != zp {
			t.Errorf("expected return object %q instead of %q", zp, zz)
		}
		{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
	for k, r := range []Rule{r1, r2} {
		for x := 0; x < 5; x++ {
			_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
			if e != p {
				t.Errorf("expected event %q instead of %q", p, e)
			}
			{{if .Type "Z" -}}
			if zz != zp {
				t.Errorf("expected return object %q instead of %q", zp, zz)
			}
			{{end -}}
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
	{{if .Type "Z" -}}
	var zp = {{.Prototype "Z"}}
	{{end -}}
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
				_, e, {{.Ref "Z" "zz," -}} err := r(tc.ctx, p, {{.Ref "Z" "zp," -}} nil, chainCounter(&j, ChainIdentity))
				if e != p {
					t.Errorf("test case %d failed: expected event %q instead of %q", ti, p, e)
				}
				{{if .Type "Z" -}}
				if zz != zp {
					t.Errorf("expected return object %q instead of %q", zp, zz)
				}
				{{end -}}
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
	_, e, {{.Ref "Z" "zz," -}} err := r(ctx, p, {{.Ref "Z" "zp," -}} nil, ChainIdentity)
	if e != p {
		t.Errorf("expected event %q instead of %q", p, e)
	}
	{{if .Type "Z" -}}
	if zz != zp {
		t.Errorf("expected return object %q instead of %q", zp, zz)
	}
	{{end -}}
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
		Rule(Rule(nil).Eval).RateLimit(nil, OverflowWait, nil).Eval(ctx, p, {{.Ref "Z" "zp," -}} nil, ChainIdentity)
	}()
	if !didPanic {
		t.Error("expected panic because we configured a rule to deadlock")
	}
}
`))
