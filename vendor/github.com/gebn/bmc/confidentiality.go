package bmc

import (
	"fmt"

	"github.com/gebn/bmc/pkg/ipmi"
	"github.com/gebn/bmc/pkg/layerexts"
)

func algorithmCipher(a ipmi.ConfidentialityAlgorithm, g AdditionalKeyMaterialGenerator) (layerexts.SerializableDecodingLayer, error) {
	switch a {
	case ipmi.ConfidentialityAlgorithmNone:
		return nil, nil
	case ipmi.ConfidentialityAlgorithmAESCBC128:
		key := [16]byte{}
		copy(key[:], g.K(2))
		return ipmi.NewAES128CBC(key)
	default:
		return nil, fmt.Errorf("unsupported confidentiality algorithm: %v", a)
	}
}
