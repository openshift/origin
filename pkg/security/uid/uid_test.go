package uid

import (
	"strings"
	"testing"
)

func TestParseRange(t *testing.T) {
	testCases := map[string]struct {
		in    string
		errFn func(error) bool
		r     Range
		total uint32
	}{
		"identity range": {
			in: "1-1/1",
			r: Range{
				block: Block{1, 1},
				size:  1,
			},
			total: 1,
		},
		"simple range": {
			in: "1-2/1",
			r: Range{
				block: Block{1, 2},
				size:  1,
			},
			total: 2,
		},
		"wide range": {
			in: "10000-999999/1000",
			r: Range{
				block: Block{10000, 999999},
				size:  1000,
			},
			total: 990,
		},
		"overflow uint": {
			in:    "1000-100000000000000/1",
			errFn: func(err error) bool { return strings.Contains(err.Error(), "unsigned integer overflow") },
		},
		"negative range": {
			in:    "1000-999/1",
			errFn: func(err error) bool { return strings.Contains(err.Error(), "must be less than end 999") },
		},
		"zero block size": {
			in:    "1000-1000/0",
			errFn: func(err error) bool { return strings.Contains(err.Error(), "block size must be a positive integer") },
		},
		"large block size": {
			in:    "1000-1001/3",
			errFn: func(err error) bool { return strings.Contains(err.Error(), "must be less than or equal to the range") },
		},
	}

	for s, testCase := range testCases {
		r, err := ParseRange(testCase.in)
		if testCase.errFn != nil && !testCase.errFn(err) {
			t.Errorf("%s: unexpected error: %v", s, err)
			continue
		}
		if err != nil {
			continue
		}
		if r.block.Start != testCase.r.block.Start || r.block.End != testCase.r.block.End || r.size != testCase.r.size {
			t.Errorf("%s: unexpected range: %#v", s, r)
		}
		if r.Size() != testCase.total {
			t.Errorf("%s: unexpected total: %d", s, r.Size())
		}
	}
}

func TestOffset(t *testing.T) {
	testCases := map[string]struct {
		r         Range
		block     Block
		contained bool
		offset    uint32
	}{
		"identity range": {
			r: Range{
				block: Block{1, 1},
				size:  1,
			},
			block:     Block{1, 1},
			contained: true,
		},
		"out of identity range": {
			r: Range{
				block: Block{1, 1},
				size:  1,
			},
			block: Block{2, 2},
		},
		"out of identity range expanded": {
			r: Range{
				block: Block{1, 1},
				size:  1,
			},
			block: Block{2, 3},
		},
		"aligned to offset": {
			r: Range{
				block: Block{0, 100},
				size:  10,
			},
			block:     Block{10, 19},
			contained: true,
			offset:    1,
		},
		"not aligned": {
			r: Range{
				block: Block{0, 100},
				size:  10,
			},
			block: Block{11, 20},
		},
	}

	for s, testCase := range testCases {
		contained, offset := testCase.r.Offset(testCase.block)
		if contained != testCase.contained {
			t.Errorf("%s: unexpected contained: %t", s, contained)
			continue
		}
		if offset != testCase.offset {
			t.Errorf("%s: unexpected offset: %d", s, offset)
			continue
		}
		if contained {
			block, ok := testCase.r.BlockAt(offset)
			if !ok {
				t.Errorf("%s: should find block", s)
				continue
			}
			if block != testCase.block {
				t.Errorf("%s: blocks are not equivalent: %#v", s, block)
			}
		}
	}
}
