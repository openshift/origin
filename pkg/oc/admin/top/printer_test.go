package top

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintSize(t *testing.T) {
	testCases := []struct {
		in  int64
		out string
	}{
		{int64(1024), "0MiB"},
		{int64(10 * 1024), "0.01MiB"},
		{int64(1024 * 1024), "1MiB"},
		{int64(100 * 1024 * 1024), "100MiB"},
		{int64((100 * 1024 * 1024) + (10 * 1024)), "100.01MiB"},
		{int64((999 * 1024 * 1024) + (999 * 1024)), "999.99MiB"},
		{int64(1024 * 1024 * 1024), "1GiB"},
		{int64((2 * 1024 * 1024 * 1024) + (500 * 1024 * 1024)), "2.50GiB"},
	}

	var b bytes.Buffer
	for idx, test := range testCases {
		printSize(&b, test.in)
		actual := strings.TrimSpace(b.String())
		if actual != test.out {
			t.Errorf("%d: unexpected output: got %s, expected %s", idx, actual, test.out)
		}
		b.Reset()
	}
}
