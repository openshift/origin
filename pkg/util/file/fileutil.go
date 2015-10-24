package file

import (
	"bufio"
	"bytes"
	"io"
	"os"
)

// ReadLines reads the content of the given file into a string slice
func ReadLines(fileName string) ([]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

type crlfWriter struct {
	io.Writer
}

func NewCRLFWriter(w io.Writer) io.Writer {
	return crlfWriter{w}
}

func (w crlfWriter) Write(b []byte) (n int, err error) {
	for i, written := 0, 0; ; {
		next := bytes.Index(b[i:], []byte("\n"))
		if next == -1 {
			n, err := w.Writer.Write(b[i:])
			return written + n, err
		}
		next = next + i
		n, err := w.Writer.Write(b[i:next])
		if err != nil {
			return written + n, err
		}
		written += n
		n, err = w.Writer.Write([]byte("\r\n"))
		if err != nil {
			if n > 1 {
				n = 1
			}
			return written + n, err
		}
		written += 1
		i = next + 1
	}
}
