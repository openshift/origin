package aesgcm_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"io"
	"testing"

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/uuid"

	"github.com/openshift/origin/pkg/cmd/server/crypto/codec/aesgcm"
)

type testEncoder struct {
	obj  runtime.Object
	data []byte
	err  error
}

func (e *testEncoder) Encode(obj runtime.Object, w io.Writer) error {
	e.obj = obj
	w.Write(e.data)
	return e.err
}

func TestEncrypt(t *testing.T) {
	block, err := aes.NewCipher([]byte("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte("test:")
	encoder := &testEncoder{data: []byte("block")}
	secret := &v1.Secret{ObjectMeta: v1.ObjectMeta{Name: "test", Namespace: "namespace", UID: uuid.NewUUID()}}
	s := aesgcm.NewEncryptingSerializer(prefix, block, encoder, nil)
	buf := &bytes.Buffer{}
	if err := s.Encode(secret, buf); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if !bytes.HasPrefix(data, prefix) {
		t.Fatalf("data doesn't start with prefix:\n%s", hex.Dump(data))
	}
	t.Logf("data:\n%s", hex.Dump(data))

	data = bytes.TrimPrefix(data, prefix)
	i := bytes.IndexRune(data, ':')
	uid := data[:i]
	data = data[i+1:]
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	result, err := aead.Open(nil, uid[:aead.NonceSize()], data, uid)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoder.data, result) {
		t.Fatalf("encoded body did not match:\n%s", hex.Dump(result))
	}
}
