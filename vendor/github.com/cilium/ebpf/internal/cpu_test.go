package internal

import (
	"io"
	"io/ioutil"
	"testing"
)

func TestParseCPUs(t *testing.T) {
	for str, result := range map[string]int{
		"0-1": 2,
		"0":   1,
	} {
		fh, err := ioutil.TempFile("", "ebpf")
		if err != nil {
			t.Fatal(err)
		}

		if _, err := io.WriteString(fh, str); err != nil {
			t.Fatal(err)
		}
		fh.Close()

		n, err := parseCPUs(fh.Name())
		if err != nil {
			t.Error("Can't parse", str, err)
		} else if n != result {
			t.Error("Parsing", str, "returns", n, "instead of", result)
		}
	}
}
