package tarlog

import (
	"archive/tar"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTarLogger(t *testing.T) {
	cases := make([]struct {
		name string
		data []byte
	}, 32)
	for i := range cases {
		cases[i].name = "ext"
		if i > 0 {
			cases[i].name = cases[i-1].name + "." + cases[i].name
		}
		cases[i].data = make([]byte, i*64)
	}

	loggedNames := []string{}
	logNames := func(h *tar.Header) {
		loggedNames = append(loggedNames, h.Name)
	}

	logger, err := NewLogger(logNames)
	require.NoError(t, err, "error creating new TarLogger")

	writer := tar.NewWriter(logger)
	for i := range cases {
		h := &tar.Header{
			Name:     cases[i].name,
			Typeflag: tar.TypeReg,
			Size:     int64(len(cases[i].data)),
		}
		err := writer.WriteHeader(h)
		require.NoError(t, err, "error writing header to tar buffer")
		n, err := writer.Write(cases[i].data)
		require.NoError(t, err, "error writing data to tar buffer")
		require.Equal(t, n, len(cases[i].data), "expected to write %d bytes, wrote %d", len(cases[i].data), n)
	}
	writer.Close()

	logger.Close()

	require.Equal(t, len(loggedNames), len(cases), "expected to log %d names, logged %d", len(cases), len(loggedNames))
	for i := range cases {
		require.Equal(t, loggedNames[i], cases[i].name, "expected to see name %q, got name %q", cases[i].name, loggedNames[i])
	}
}
