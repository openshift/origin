package apprclient

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeHappyPath(t *testing.T) {
	expected := "message in a bottle"
	content := compress(t, expected)

	var d blobDecoder = &blobDecoderImpl{}
	decoded, err := d.Decode(content)

	assert.Nil(t, err)
	assert.Equal(t, string(expected), string(decoded))
}

func base64Decode(encoded []byte) ([]byte, error) {
	maxlength := base64.StdEncoding.DecodedLen(len(encoded))
	decoded := make([]byte, maxlength)

	n, err := base64.StdEncoding.Decode(decoded, encoded)
	if err != nil {
		return nil, err
	}

	return decoded[:n], nil
}

func compress(t *testing.T, message string) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	defer w.Close()

	_, err := w.Write([]byte(message))
	require.NoError(t, err)

	// Note: we need to call Close before we start using the gzip stream.
	err = w.Close()
	require.NoError(t, err)

	return b.Bytes()
}
