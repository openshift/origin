package bmc

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"

	"github.com/gebn/bmc/pkg/ipmi"
)

// algorithmHasher creates a Hash from the provided IPMI V2.0 algorithm, to be
// used to sign packets with the Authenticated flag set to true. Note that not
// all algorithms are authenticated, e.g. MD5-128.
func algorithmHasher(i ipmi.IntegrityAlgorithm, g AdditionalKeyMaterialGenerator) (hash.Hash, error) {
	switch i {
	case ipmi.IntegrityAlgorithmNone:
		return nil, nil
	case ipmi.IntegrityAlgorithmHMACSHA196:
		return &truncatedHash{
			Hash:   hmac.New(sha1.New, g.K(1)),
			length: 12,
		}, nil
	case ipmi.IntegrityAlgorithmHMACMD5128:
		return hmac.New(md5.New, g.K(1)), nil
	//case ipmi.IntegrityAlgorithmMD5128:
	//    // this is a special case, needing the user password
	//    // TODO implement if needed
	case ipmi.IntegrityAlgorithmHMACSHA256128:
		return truncatedHash{
			Hash:   hmac.New(sha256.New, g.K(1)),
			length: 16,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported integrity algorithm: %v", i)
	}
}
