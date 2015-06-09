package mcs

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const maxCategories = 1024

type Label struct {
	Prefix string
	Categories
}

// NewLabel creates a Label object based on the offset given by
// offset with a number of labels equal to k. Prefix may be any
// valid SELinux label (user:role:type:level:).
func NewLabel(prefix string, offset uint64, k uint) (*Label, error) {
	if len(prefix) > 0 && !(strings.HasSuffix(prefix, ":") || strings.HasSuffix(prefix, ",")) {
		prefix = prefix + ":"
	}
	return &Label{
		Prefix:     prefix,
		Categories: categoriesForOffset(offset, maxCategories, k),
	}, nil
}

// ParseLabel converts a string value representing an SELinux label
// into a Label object, extracting and ordering categories.
func ParseLabel(in string) (*Label, error) {
	if len(in) == 0 {
		return &Label{}, nil
	}

	prefix := strings.Split(in, ":")
	segment := prefix[len(prefix)-1]
	if len(prefix) > 0 {
		prefix = prefix[:len(prefix)-1]
	}
	prefixString := strings.Join(prefix, ":")
	if len(prefixString) > 0 {
		prefixString += ":"
	}

	var categories Categories
	for _, s := range strings.Split(segment, ",") {
		if !strings.HasPrefix(s, "c") {
			return nil, fmt.Errorf("categories must start with 'c': %s", segment)
		}
		i, err := strconv.Atoi(s[1:])
		if err != nil {
			return nil, err
		}
		categories = append(categories, uint16(i))
	}
	sort.Sort(categories)

	last := -1
	for _, c := range categories {
		if int(c) == last {
			return nil, fmt.Errorf("labels may not contain duplicate categories: %s", in)
		}
		last = int(c)
	}

	return &Label{
		Prefix:     prefixString,
		Categories: categories,
	}, nil
}

func (labels *Label) String() string {
	buf := bytes.Buffer{}
	buf.WriteString(labels.Prefix)
	for i, label := range labels.Categories {
		if i != 0 {
			buf.WriteRune(',')
		}
		buf.WriteRune('c')
		buf.WriteString(strconv.Itoa(int(label)))
	}
	return buf.String()
}

// Offset returns the rank of the provided categories in the
// co-lex rank operation (k is implicit)
func (categories Categories) Offset() uint64 {
	k := len(categories)
	r := uint64(0)
	for i := 0; i < k; i++ {
		r += binomial(uint(categories[i]), uint(k-i))
	}
	return r
}

// categoriesForOffset calculates the co-lex unrank operation
// on the combinatorial group defined by n, k, where rank is
// the offset. n is typically 1024 (the SELinux max)
func categoriesForOffset(offset uint64, n, k uint) Categories {
	var categories Categories
	for i := uint(0); i < k; i++ {
		current := binomial(n, k-i)
		for current > offset {
			n--
			current = binomial(n, k-i)
		}
		categories = append(categories, uint16(n))
		offset = offset - current
	}
	sort.Sort(categories)
	return categories
}

type Categories []uint16

func (c Categories) Len() int      { return len(c) }
func (c Categories) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c Categories) Less(i, j int) bool {
	return c[i] > c[j]
}

func binomial(n, k uint) uint64 {
	if n < k {
		return 0
	}
	if k == n {
		return 1
	}
	r := uint64(1)
	for d := uint(1); d <= k; d++ {
		r *= uint64(n)
		r /= uint64(d)
		n--
	}
	return r
}

type Range struct {
	prefix string
	n      uint
	k      uint
}

// NewRange describes an SELinux category range, where prefix may include
// the user, type, role, and level of the range, and n and k represent the
// highest category c0 to c(N-1) and k represents the number of labels to use.
// A range can be used to check whether a given label matches the range.
func NewRange(prefix string, n, k uint) (*Range, error) {
	if n == 0 {
		return nil, fmt.Errorf("label max value must be a positive integer")
	}
	if k == 0 {
		return nil, fmt.Errorf("label length must be a positive integer")
	}
	return &Range{
		prefix: prefix,
		n:      n,
		k:      k,
	}, nil
}

// ParseRange converts a string value representing an SELinux category
// range into a Range object, extracting the prefix -- which may include the
// user, type, and role of the range, the number of labels to use, and the
// maximum category to use.  The input string is expected to be in the format:
//
//   <prefix>/<numLabels>[,<maxCategory>]
//
// If the maximum category is not specified, it is defaulted to the maximum
// number of SELinux categories (1024).
func ParseRange(in string) (*Range, error) {
	seg := strings.SplitN(in, "/", 2)
	if len(seg) != 2 {
		return nil, fmt.Errorf("range not in the format \"<prefix>/<numLabel>[,<maxCategory>]\"")
	}
	prefix := seg[0]
	n := maxCategories
	size := strings.SplitN(seg[1], ",", 2)
	k, err := strconv.Atoi(size[0])
	if err != nil {
		return nil, fmt.Errorf("range not in the format \"<prefix>/<numLabel>[,<maxCategory>]\"")
	}
	if len(size) > 1 {
		max, err := strconv.Atoi(size[1])
		if err != nil {
			return nil, fmt.Errorf("range not in the format \"<prefix>/<numLabel>[,<maxCategory>]\"")
		}
		n = max
	}
	if k > 5 {
		return nil, fmt.Errorf("range may not include more than 5 labels")
	}
	if n > maxCategories {
		return nil, fmt.Errorf("range may not include more than %d categories", maxCategories)
	}
	return NewRange(prefix, uint(n), uint(k))
}

func (r *Range) Size() uint64 {
	return binomial(r.n, uint(r.k))
}

func (r *Range) String() string {
	if r.n == maxCategories {
		return fmt.Sprintf("%s/%d", r.prefix, r.k)
	}
	return fmt.Sprintf("%s/%d,%d", r.prefix, r.k, r.n)
}

func (r *Range) LabelAt(offset uint64) (*Label, bool) {
	label, err := NewLabel(r.prefix, offset, r.k)
	return label, err == nil
}

func (r *Range) Contains(label *Label) bool {
	if label.Prefix != r.prefix {
		return false
	}
	if len(label.Categories) != int(r.k) {
		return false
	}
	for _, i := range label.Categories {
		if i >= uint16(r.n) {
			return false
		}
	}
	return true
}

func (r *Range) Offset(label *Label) (bool, uint64) {
	if !r.Contains(label) {
		return false, 0
	}
	return true, label.Offset()
}
