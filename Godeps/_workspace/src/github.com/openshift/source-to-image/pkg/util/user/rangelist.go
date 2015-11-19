package user

import (
	"regexp"
	"strconv"
	"strings"
)

// RangeList is a list of user ranges
type RangeList []*Range

// ParseRangeList parses a string that contains a comma-separated list of ranges
func ParseRangeList(str string) (*RangeList, error) {
	rl := RangeList{}
	if len(str) == 0 {
		return &rl, nil
	}
	parts := strings.Split(str, ",")
	for _, p := range parts {
		r, err := ParseRange(p)
		if err != nil {
			return nil, err
		}
		rl = append(rl, r)
	}
	return &rl, nil
}

// Empty returns true if the RangeList is empty
func (l *RangeList) Empty() bool {
	if len(*l) == 0 {
		return true
	}
	for _, r := range *l {
		if !r.Empty() {
			return false
		}
	}
	return true
}

// Contains returns true if the uid is contained by any range in the RangeList
func (l *RangeList) Contains(uid int) bool {
	for _, r := range *l {
		if r.Contains(uid) {
			return true
		}
	}
	return false
}

// Type returns the type of a RangeList object
func (l *RangeList) Type() string {
	return "user.RangeList"
}

// Set sets the value of a RangeList object
func (l *RangeList) Set(value string) error {
	newRangeList, err := ParseRangeList(value)
	if err != nil {
		return err
	}
	*l = *newRangeList
	return nil
}

// String returns a parseable string representation of a RangeList
func (l *RangeList) String() string {
	rangeStrings := []string{}
	for _, r := range *l {
		rangeStrings = append(rangeStrings, r.String())
	}
	return strings.Join(rangeStrings, ",")
}

// IsUserAllowed checks that the given user is numeric and is
// contained by the given RangeList
func IsUserAllowed(user string, allowed *RangeList) bool {
	uid, err := strconv.Atoi(user)
	if err != nil {
		return false
	}
	return allowed.Contains(uid)
}

var dockerLineDelim = regexp.MustCompile(`[\t\v\f\r ]+`)

// IsOnbuildAllowed checks a list of Docker ONBUILD instructions for
// user directives. It ensures that any users specified by the directives
// falls within the specified range list of users.
func IsOnbuildAllowed(directives []string, allowed *RangeList) bool {
	for _, line := range directives {
		parts := dockerLineDelim.Split(line, 2)
		if strings.ToLower(parts[0]) != "user" {
			continue
		}
		if !IsUserAllowed(parts[1], allowed) {
			return false
		}
	}
	return true
}
