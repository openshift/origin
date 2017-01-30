package aesgcm

import (
	"bytes"
	"crypto/cipher"
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
)

type encryptingSerializer struct {
	prefix  []byte
	block   cipher.Block
	encoder runtime.Encoder
	decoder runtime.Decoder
}

func NewEncryptingSerializer(prefix []byte, block cipher.Block, encoder runtime.Encoder, decoder runtime.Decoder) runtime.Serializer {
	return &encryptingSerializer{
		prefix:  prefix,
		block:   block,
		encoder: encoder,
		decoder: decoder,
	}
}

func (c *encryptingSerializer) Encode(obj runtime.Object, w io.Writer) error {
	aead, err := cipher.NewGCM(c.block)
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	if err := c.encoder.Encode(obj, buf); err != nil {
		return err
	}
	m, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("only objects that expose object meta may be encrypted (%T): %v", obj, err)
	}
	nonce := []byte(m.GetUID())
	additionalData := nonce
	nonceSize := aead.NonceSize()
	switch l := len(nonce); {
	case l < nonceSize:
		return fmt.Errorf("UID length not long enough to pass as nonce, must be %d bytes", nonceSize)
	case l > nonceSize:
		nonce = nonce[:nonceSize]
	}
	data := buf.Bytes()
	out := aead.Seal(data[:0], nonce, data, additionalData)
	if _, err := w.Write(c.prefix); err != nil {
		return err
	}
	if _, err := w.Write(additionalData); err != nil {
		return err
	}
	if _, err := w.Write([]byte(":")); err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func (c *encryptingSerializer) Decode(data []byte, defaults *unversioned.GroupVersionKind, into runtime.Object) (runtime.Object, *unversioned.GroupVersionKind, error) {
	return nil, nil, fmt.Errorf("not implemented")
}

func (c *encryptingSerializer) RecognizesData(peek io.Reader) (ok, unknown bool, err error) {
	return false, false, fmt.Errorf("not implemented")
}
