package bmc

import (
	"bytes"
	"context"
	"fmt"

	"github.com/gebn/bmc/pkg/iana"
	"github.com/gebn/bmc/pkg/ipmi"
)

// RetrieveSupportedCipherSuites queries an IPMI v2.0 connection for cipher
// suites that can be used to establish a session.
func RetrieveSupportedCipherSuites(ctx context.Context, s *V2SessionlessTransport) ([]ipmi.CipherSuiteRecord, error) {
	// we only need a *V2Sessionless, however then this method has to be called
	// with machine.V2Sessionless, rather than just machine, which is awkward

	getChannelCipherSuitesCmd := ipmi.GetChannelCipherSuitesCmd{
		Req: ipmi.GetChannelCipherSuitesReq{
			Channel: ipmi.ChannelPresentInterface,
		},
	}

	// a given cipher suite record can span multiple list index values; to work
	// around this, retrieve all the bytes, then process them
	cipherSuiteRecordData := bytes.Buffer{}
	for {
		if err := ValidateResponse(s.SendCommand(ctx, &getChannelCipherSuitesCmd)); err != nil {
			return nil, err
		}
		cipherSuiteRecordData.Write(getChannelCipherSuitesCmd.Rsp.CipherSuiteRecordsChunk)
		if getChannelCipherSuitesCmd.Req.ListIndex == 64 ||
			len(getChannelCipherSuitesCmd.Rsp.CipherSuiteRecordsChunk) < 16 {
			break
		}
		getChannelCipherSuitesCmd.Req.ListIndex++
	}
	return parseCipherSuiteRecordData(cipherSuiteRecordData.Bytes())
}

// parseCipherSuiteRecordData interprets a buffer of adjacent Cipher Suite
// Records. A record containing multiple integrity or confidentiality
// algorithms is expended into multiple records. Returns an error if trailing
// data, as this is indistinguishable from a parse error.
func parseCipherSuiteRecordData(joined []byte) ([]ipmi.CipherSuiteRecord, error) {
	records := []ipmi.CipherSuiteRecord{}
	for len(joined) > 0 {
		if joined[0]>>1 != 0x60 {
			return nil, fmt.Errorf("expected start of record, got %#v", joined[0])
		}
		record := ipmi.CipherSuiteRecord{}
		offset := 2 // starts with authentication algorithm
		switch joined[0] & 1 {
		case 0:
			// standard suite, need at least 2 more bytes
			if len(joined) < 3 {
				return nil, fmt.Errorf("standard suite record must be at least 3 bytes, got %v", len(joined))
			}
		case 1:
			// OEM suite, need at least 5 more bytes
			if len(joined) < 6 {
				return nil, fmt.Errorf("OEM suite record must be at least 6 bytes, got %v", len(joined))
			}
			record.Enterprise = iana.Enterprise(uint32(joined[2]) + uint32(joined[3])<<8 + uint32(joined[4])<<16)
			offset += 3
		}
		// we've now done the bounds check
		record.CipherSuiteID = ipmi.CipherSuiteID(joined[1])

		if joined[offset]>>6 != 0b00 {
			return nil, fmt.Errorf("expected authentication algorithm, got %#v", joined[offset])
		}
		record.AuthenticationAlgorithm = ipmi.AuthenticationAlgorithm(joined[offset])
		offset++

		// rather than storing, we could record offsets, however it would get
		// complicated if integrity and/or confidentiality did not exist - we
		// still need to add at least one record

		integrityAlgorithms := make([]ipmi.IntegrityAlgorithm, 0, 1)
		for ; len(joined) > offset && joined[offset]>>6 == 0b01; offset++ {
			integrityAlgorithms = append(integrityAlgorithms, ipmi.IntegrityAlgorithm(joined[offset]&0x3f))
		}
		if len(integrityAlgorithms) == 0 {
			integrityAlgorithms = append(integrityAlgorithms, ipmi.IntegrityAlgorithmNone)
		}

		confidentialityAlgorithms := make([]ipmi.ConfidentialityAlgorithm, 0, 1)
		for ; len(joined) > offset && joined[offset]>>6 == 0b10; offset++ {
			confidentialityAlgorithms = append(confidentialityAlgorithms, ipmi.ConfidentialityAlgorithm(joined[offset]&0x3f))
		}
		if len(confidentialityAlgorithms) == 0 {
			confidentialityAlgorithms = append(confidentialityAlgorithms, ipmi.ConfidentialityAlgorithmNone)
		}

		for _, integrityAlgorithm := range integrityAlgorithms {
			record.IntegrityAlgorithm = integrityAlgorithm
			for _, confidentialityAlgorithm := range confidentialityAlgorithms {
				record.ConfidentialityAlgorithm = confidentialityAlgorithm
				records = append(records, record)
			}
		}

		joined = joined[offset:]
	}
	return records, nil
}
