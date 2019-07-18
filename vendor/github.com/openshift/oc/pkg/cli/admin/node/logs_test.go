package node

import (
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
)

func Test_optionallyDecompress(t *testing.T) {
	longString := strings.Repeat(`some test content`, 1000)
	tests := []struct {
		name    string
		in      io.Reader
		wantOut string
		wantErr bool
	}{
		{in: gzipped(`some test content`), wantOut: `some test content`},
		{in: bytes.NewBufferString(`some test content`), wantOut: `some test content`},
		{in: gzipped(longString), wantOut: longString},
		{in: bytes.NewBufferString(longString), wantOut: longString},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := optionallyDecompress(out, tt.in); (err != nil) != tt.wantErr {
				t.Errorf("optionallyDecompress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOut := out.String(); gotOut != tt.wantOut {
				t.Errorf("optionallyDecompress() = %v, want %v", gotOut, tt.wantOut)
			}
		})
	}
}

func gzipped(s string) io.Reader {
	out := &bytes.Buffer{}
	gw := gzip.NewWriter(out)
	gw.Write([]byte(s))
	gw.Close()
	return out
}

func Test_outputDirectoryEntriesOrContent(t *testing.T) {
	tests := []struct {
		name    string
		in      io.Reader
		prefix  []byte
		wantOut string
		wantErr bool
	}{
		{in: bytes.NewBufferString(`<pre><a href="line">`), wantOut: "line\n"},
		{in: bytes.NewBufferString(`<pre><a href="line">\n`), wantOut: "line\n"},
		{prefix: []byte("test: "), in: bytes.NewBufferString(`<pre><a href="line">\n`), wantOut: "test: line\n"},
		{in: bytes.NewBufferString(`<pre><a href="">\n`), wantOut: ""},
		{in: bytes.NewBufferString(`<pre><a href="`), wantOut: ""},
		{in: bytes.NewBufferString(``), wantOut: ""},
		{in: bytes.NewBufferString(` <pre><a href="line">`), wantOut: ` <pre><a href="line">`},

		{in: bytes.NewBufferString("<pre>" + strings.Repeat("<a href=\"link\">stuff</a>\n", 1000)), wantOut: strings.Repeat("link\n", 1000)},
		{prefix: []byte("test: "), in: bytes.NewBufferString("<pre>" + strings.Repeat("<a href=\"link\">stuff</a>\n", 1000)), wantOut: strings.Repeat("test: link\n", 1000)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			if err := outputDirectoryEntriesOrContent(out, tt.in, tt.prefix); (err != nil) != tt.wantErr {
				t.Errorf("outputDirectoryEntriesOrContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOut := out.String(); gotOut != tt.wantOut {
				t.Errorf("outputDirectoryEntriesOrContent() = %v, want %v", gotOut, tt.wantOut)
			}
		})
	}
}

func Test_mergeReader_WriteTo(t *testing.T) {
	tests := []struct {
		name    string
		in      []Reader
		wantOut string
		wantErr bool
	}{
		{in: nil, wantOut: ""},
		{in: readers("1\n2\n3\n"), wantOut: "1\n2\n3\n"},
		{in: readers("1\n2", "1\n3\n"), wantOut: "1\n1\n2\n3\n"},
		{in: readers("1a\n2a\n3a\n", "2b\n3b\n4b\n", "1c\n3c\n4c\n"), wantOut: "1a\n1c\n2a\n2b\n3a\n3b\n3c\n4b\n4c\n"},

		{in: readers("a|1\n2\n3\n"), wantOut: "a1\na2\na3\n"},
		{in: readers("a|1\n2", "b|1\n3\n"), wantOut: "a1\nb1\na2\nb3\n"},
		{in: readers("a: |1\n2", "b: |1\n3\n"), wantOut: "a: 1\nb: 1\na: 2\nb: 3\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := mergeReader(tt.in)
			out := &bytes.Buffer{}
			n, err := r.WriteTo(out)
			if (err != nil) != tt.wantErr {
				t.Errorf("mergeReader.WriteTo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if n != int64(out.Len()) {
				t.Errorf("mergeReader.WriteTo() = %v, want %v", n, out.Len())
			}
			if gotOut := out.String(); gotOut != tt.wantOut {
				t.Errorf("mergeReader.WriteTo() = %v, want %v", gotOut, tt.wantOut)
			}
		})
	}
}

func readers(all ...string) []Reader {
	var out []Reader
	for _, s := range all {
		if strings.Contains(s, "|") {
			parts := strings.SplitN(s, "|", 2)
			out = append(out, Reader{Prefix: []byte(parts[0]), R: bytes.NewBufferString(parts[1])})
		} else {
			out = append(out, Reader{R: bytes.NewBufferString(s)})
		}
	}
	return out
}
