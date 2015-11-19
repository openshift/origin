package user

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Range errors
var (
	ErrInvalidRange = errors.New("invalid range; a range must consist of positive integers and the upper bound must be greater than or equal to the lower bound")
)

// ErrParseRange is an error encountered while parsing a Range
type ErrParseRange struct {
	cause error
}

func (e *ErrParseRange) Error() string {
	msg := "error parsing range; a range must be of one of the following formats: [n], [-n], [n-], [n:m] where n and m are positive numbers and m is greater than or equal to n"
	if e.cause != nil {
		msg = fmt.Sprintf("%s: %v", msg, e.cause)
	}
	return msg
}

// Range represents a range of user ids. It can be unbound at either end with a nil value
// I both From and To are present, To must be greater than or equal to From. Bounds are inclusive
type Range struct {
	from *int
	to   *int
}

// NewRange creates a new range with lower and upper bound
func NewRange(from int, to int) (*Range, error) {
	return (&rangeBuilder{}).from(from, nil).to(to, nil).Range()
}

// NewRangeTo creates a new range with only the upper bound
func NewRangeTo(to int) (*Range, error) {
	return (&rangeBuilder{}).to(to, nil).Range()
}

// NewRangeFrom creates a new range with only the lower bound
func NewRangeFrom(from int) (*Range, error) {
	return (&rangeBuilder{}).from(from, nil).Range()
}

func parseInt(str string) (int, error) {
	num, err := strconv.Atoi(str)
	if err != nil {
		return 0, &ErrParseRange{cause: err}
	}
	return num, nil
}

type rangeBuilder struct {
	r   Range
	err error
}

func (b *rangeBuilder) from(num int, err error) *rangeBuilder {
	return b.setBound(num, err, &b.r.from)
}

func (b *rangeBuilder) to(num int, err error) *rangeBuilder {
	return b.setBound(num, err, &b.r.to)
}

func (b *rangeBuilder) setBound(num int, err error, bound **int) *rangeBuilder {
	if b.err != nil {
		return b
	}
	if b.err = err; b.err != nil {
		return b
	}
	if num < 0 {
		b.err = ErrInvalidRange
		return b
	}
	*bound = &num
	return b
}

func (b *rangeBuilder) Range() (*Range, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.r.from != nil && b.r.to != nil && *b.r.to < *b.r.from {
		return nil, ErrInvalidRange
	}
	return &b.r, nil
}

// ParseRange creates a Range from a given string
func ParseRange(value string) (*Range, error) {
	value = strings.TrimSpace(value)
	b := &rangeBuilder{}
	if value == "" {
		return b.Range()
	}
	parts := strings.Split(value, "-")
	switch len(parts) {
	case 1:
		num, err := parseInt(parts[0])
		return b.from(num, err).to(num, err).Range()
	case 2:
		if parts[0] != "" {
			b.from(parseInt(parts[0]))
		}
		if parts[1] != "" {
			b.to(parseInt(parts[1]))
		}
		return b.Range()
	default:
		return nil, &ErrParseRange{}
	}
}

// Contains returns true if the argument falls inside the Range
func (r *Range) Contains(value int) bool {
	if r.from == nil && r.to == nil {
		return false
	}
	if r.from != nil && value < *r.from {
		return false
	}
	if r.to != nil && value > *r.to {
		return false
	}
	return true
}

// String returns a parse-able string representation of a Range
func (r *Range) String() string {
	switch {
	case r.from == nil && r.to == nil:
		return ""
	case r.from == nil:
		return fmt.Sprintf("-%d", *r.to)
	case r.to == nil:
		return fmt.Sprintf("%d-", *r.from)
	case *r.from == *r.to:
		return fmt.Sprintf("%d", *r.to)
	default:
		return fmt.Sprintf("%d-%d", *r.from, *r.to)
	}
}

// Type returns the type of a Range object
func (r *Range) Type() string {
	return "user.Range"
}

// Set sets the value of a Range object
func (r *Range) Set(value string) error {
	newRange, err := ParseRange(value)
	if err != nil {
		return err
	}
	*r = *newRange
	return nil
}

// Empty returns true if the range has no bounds
func (r *Range) Empty() bool {
	return r.from == nil && r.to == nil
}
