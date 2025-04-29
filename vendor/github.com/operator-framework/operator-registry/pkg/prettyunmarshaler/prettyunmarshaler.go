package prettyunmarshaler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type JSONUnmarshalError struct {
	data   []byte
	offset int64
	err    error
}

func NewJSONUnmarshalError(data []byte, err error) *JSONUnmarshalError {
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) {
		return &JSONUnmarshalError{data: data, offset: te.Offset, err: te}
	}
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return &JSONUnmarshalError{data: data, offset: se.Offset, err: se}
	}
	return &JSONUnmarshalError{data: data, offset: -1, err: err}
}

func (e *JSONUnmarshalError) Error() string {
	return e.err.Error()
}

func (e *JSONUnmarshalError) Pretty() string {
	if len(e.data) == 0 || e.offset < 0 || e.offset > int64(len(e.data)) {
		return e.err.Error()
	}

	const marker = " <=="

	var sb strings.Builder
	_, _ = sb.WriteString(fmt.Sprintf("%s at offset %d (indicated by%s)\n", e.err.Error(), e.offset, marker))

	prettyBuf := bytes.NewBuffer(make([]byte, 0, len(e.data)))
	err := json.Indent(prettyBuf, e.data, "", "    ")

	// If there was an error indenting the JSON, just treat the original data as the pretty data.
	if err != nil {
		prettyBuf = bytes.NewBuffer(e.data)
	}

	// If the offset is at the end of the data, just print the pretty data and the marker at the end.
	if int(e.offset) == len(e.data) {
		_, _ = sb.WriteString(prettyBuf.String())
		_, _ = sb.WriteString(marker)
		return sb.String()
	}

	// If the offset is within the data, find the corresponding offset in the pretty data.
	var (
		pIndex  int
		pOffset int
	)
	pretty := prettyBuf.Bytes()
	for dIndex, b := range e.data {
		// If we've reached the offset, record it and break out of the loop
		if dIndex == int(e.offset) {
			pOffset = pIndex
			break
		}

		// Fast-forward the pretty index until we find the byte in the pretty data
		// that matches the byte in the original data.
		for pretty[pIndex] != b {
			pIndex++
			if pIndex >= len(pretty) {
				// Something went wrong. For example, if the pretty data somehow reordered
				// the bytes or is missing a byte
				return e.err.Error()
			}
		}

		// We found the byte in the pretty data that matches the byte in the original data,
		// so increment the pretty index.
		pIndex++
	}

	_, _ = sb.Write(pretty[:pOffset])
	_, _ = sb.WriteString(fmt.Sprintf("%s ", marker))
	_, _ = sb.Write(pretty[pOffset:])

	return sb.String()
}
