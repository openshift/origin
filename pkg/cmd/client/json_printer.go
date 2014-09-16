package client

import (
	"encoding/json"
	"fmt"
	"io"
)

// JSONPrinter is an implementation of ResourcePrinter which parsess raw JSON, and
// re-formats as indented, human-readable JSON.
type JSONPrinter struct{}

// Print parses the data as JSON, re-formats as JSON and prints the indented
// JSON.
func (y *JSONPrinter) Print(data []byte, w io.Writer) error {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	output, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(w, string(output))
	return err
}
