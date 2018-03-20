package haproxy

import (
	"bytes"
	"encoding/csv"
	"io"

	"github.com/gocarina/gocsv"
	"github.com/golang/glog"
)

// Converter transforms a set of bytes. The haproxy dynamic API command
// responses are not always csv/compliant. This allows us to inject custom
// converters to make the responses valid csv and parseable.
type Converter interface {
	// Convert converts a set of bytes.
	Convert(data []byte) ([]byte, error)
}

// ByteConverterFunc converts bytes!
type ByteConverterFunc func([]byte) ([]byte, error)

// CSVConverter is used to convert the haproxy dynamic configuration API
// responses into something that is valid CSV and then parse the response
// and unmarshal it into native golang structs.
type CSVConverter struct {
	headers       []byte
	out           interface{}
	converterFunc ByteConverterFunc
}

// NewCSVConverter returns a new CSVConverter.
func NewCSVConverter(headers string, out interface{}, fn ByteConverterFunc) *CSVConverter {
	return &CSVConverter{
		headers:       []byte(headers),
		out:           out,
		converterFunc: fn,
	}
}

// Convert runs a haproxy dynamic config API command.
func (c *CSVConverter) Convert(data []byte) ([]byte, error) {
	glog.V(5).Infof("CSV converter input data bytes: %s", string(data))
	if c.converterFunc != nil {
		convertedBytes, err := c.converterFunc(data)
		if err != nil {
			return data, err
		}
		data = convertedBytes
		glog.V(5).Infof("CSV converter transformed data bytes: %s", string(data))
	}

	if c.out == nil {
		return data, nil
	}

	// Have an output data structure, so use CSV Reader to populate it.
	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		// Allow quotes
		r.LazyQuotes = true
		r.TrimLeadingSpace = true
		// Allows use space as delimiter
		r.Comma = ' '
		return r
	})

	glog.V(5).Infof("CSV converter fixing up csv header ...")
	data, _ = fixupHeaders(data, c.headers)
	glog.V(5).Infof("CSV converter fixed up data bytes: %s", string(data))
	return data, gocsv.Unmarshal(bytes.NewBuffer(data), c.out)
}

// fixupHeaders fixes up haproxy API responses that don't contain any CSV
// header information. This allows us to easily parse the data and marshal
// into an array of native golang structs.
func fixupHeaders(data, headers []byte) ([]byte, error) {
	prefix := []byte("#")
	if len(headers) > 0 && !bytes.HasPrefix(data, prefix) {
		// No header, so insert one.
		line := bytes.Join([][]byte{prefix, headers}, []byte(" "))
		data = bytes.Join([][]byte{line, data}, []byte("\n"))
	}

	// strip off '#', as gocsv treats the first line as csv header info.
	return bytes.TrimPrefix(data, prefix), nil
}
