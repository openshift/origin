package certs

import (
	"bytes"
	"encoding/pem"
	"os"
	"path/filepath"
)

const (
	// StringSourceEncryptedBlockType is the PEM block type used to store an encrypted string
	StringSourceEncryptedBlockType = "ENCRYPTED STRING"
	// StringSourceKeyBlockType is the PEM block type used to store an encrypting key
	StringSourceKeyBlockType = "ENCRYPTING KEY"
)

func BlockFromFile(path string, blockType string) (*pem.Block, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	block, ok := BlockFromBytes(data, blockType)
	return block, ok, nil
}

func BlockFromBytes(data []byte, blockType string) (*pem.Block, bool) {
	for {
		block, remaining := pem.Decode(data)
		if block == nil {
			return nil, false
		}
		if block.Type == blockType {
			return block, true
		}
		data = remaining
	}
}

func BlockToFile(path string, block *pem.Block, mode os.FileMode) error {
	b, err := BlockToBytes(block)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), os.FileMode(0755)); err != nil {
		return err
	}
	return os.WriteFile(path, b, mode)
}

func BlockToBytes(block *pem.Block) ([]byte, error) {
	b := bytes.Buffer{}
	if err := pem.Encode(&b, block); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
