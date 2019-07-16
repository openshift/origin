package apprclient

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

type blobDecoder interface {
	// Decode decodes package blob into plain unencrypted byte array
	Decode(encoded []byte) ([]byte, error)
}

type blobDecoderImpl struct {
}

// Decode decompresses the downloaded content from appregistry server and
// returns a byte array.
func (*blobDecoderImpl) Decode(encoded []byte) (decoded []byte, err error) {
	gzipReader, err := gzip.NewReader(bytes.NewBuffer(encoded))
	if err != nil {
		return
	}

	defer gzipReader.Close()

	decoded, err = ioutil.ReadAll(gzipReader)
	if err != nil {
		return
	}

	return
}
