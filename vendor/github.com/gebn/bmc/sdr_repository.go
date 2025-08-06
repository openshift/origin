package bmc

import (
	"context"
	"errors"
	"fmt"

	"github.com/gebn/bmc/pkg/ipmi"

	"github.com/cenkalti/backoff/v4"
	"github.com/google/gopacket"
)

const (
	sdrHeaderLength = 5
	sdrMaxLength    = 64
)

var (
	errSDRRepositoryModified = errors.New(
		"the SDR Repository was modified during enumeration")
)

// SDRRepository is a retrieved SDR Repository. For the time being, this is a
// collection of Full Sensor Records, indexed by record ID. Note that because
// this is a map, iteration order is randomised and almost definitely not the
// same as retrieval order, which has no guarantees anyway.
type SDRRepository map[ipmi.RecordID]*ipmi.FullSensorRecord

// RetrieveSDRRepository enumerates all Full Sensor Records in the BMC's SDR
// Repository. This method will back-off if an error occurs, or it detects a
// change mid-way through iteration, which would invalidate records retrieved so
// far. The session-configured timeout is used for individual commands.
func RetrieveSDRRepository(ctx context.Context, s Session) (SDRRepository, error) {
	var repo *SDRRepository
	err := backoff.Retry(func() error {
		initialInfo, err := s.GetSDRRepositoryInfo(ctx)
		if err != nil {
			return err
		}
		// we could error here if unsupported SDR Repo version; no such cases
		// currently exist
		candidateRepo, err := walkSDRs(ctx, s)
		if err != nil {
			return err
		}
		finalInfo, err := s.GetSDRRepositoryInfo(ctx)
		if err != nil {
			return err
		}
		if initialInfo.LastAddition.Before(finalInfo.LastAddition) ||
			initialInfo.LastErase.Before(finalInfo.LastErase) {
			// tough luck, start again
			return errSDRRepositoryModified
		}
		repo = &candidateRepo
		return nil
	}, backoff.WithContext(backoff.NewExponentialBackOff(), ctx))
	if err != nil {
		return nil, err
	}
	return *repo, nil
}

// walkSDRs iterates over the SDR Repository. It is not concerned with the repo
// changing behind its back.
//
// For each SDR, it starts by requesting the header and inspecting the type. If
// it's a FullSensorRecord, it then requests the key fields and body. Otherwise,
// it skips to the next SDR.
// This is more expensive than reading the entire SDR at once, but it's
// resilient to BMCs that return a malformed packet when the request's Length is
// 0xff.
func walkSDRs(ctx context.Context, s Session) (SDRRepository, error) {
	repo := SDRRepository{} // we could set a size; it's a micro-optimisation
	reserveSDRRepoCmdResp, err := s.ReserveSDRRepository(ctx)
	if err != nil {
		return nil, err
	}
	getSDRCmd := &ipmi.GetSDRCmd{
		Req: ipmi.GetSDRReq{
			RecordID:      ipmi.RecordIDFirst,
			Length:        sdrHeaderLength,                     // read header only
			ReservationID: reserveSDRRepoCmdResp.ReservationID, // needed for partial reads
		},
	}

	// it's ambiguous whether we retrieve ipmi.RecordIDLast; other
	// implementations do not. The final SDR seems to have two RecordIDs - a
	// "normal" one and ipmi.RecordIDLast, so retrieving ipmi.RecordIDLast will
	// duplicate it.
	for getSDRCmd.Req.RecordID != ipmi.RecordIDLast {
		if err := ValidateResponse(s.SendCommand(ctx, getSDRCmd)); err != nil {
			return nil, err
		}
		headerPacket := gopacket.NewPacket(getSDRCmd.Rsp.Payload, ipmi.LayerTypeSDR,
			gopacket.DecodeOptions{
				Lazy: true,
				// we can't set NoCopy because we reuse getSDRCmd.Rsp
			})
		if headerPacket == nil {
			return nil, fmt.Errorf("invalid SDR: %v", getSDRCmd)
		}
		headerLayer := headerPacket.Layer(ipmi.LayerTypeSDR)
		if headerLayer == nil {
			return nil, fmt.Errorf("packet is missing SDR layer: %v", getSDRCmd)
		}
		header := headerLayer.(*ipmi.SDR)

		if header.Type == ipmi.RecordTypeFullSensor {
			if header.Length > sdrMaxLength {
				// SDR exceeds the specified max length, which means we need to implement
				// partial reading. Hopefully we'll be alright - yet to see a SDR >70 bytes
				// long - they're specified as 64 after all.
				return nil, fmt.Errorf("SDR length %d exceeds max of %d bytes: %v",
					header.Length, sdrMaxLength, getSDRCmd)
			}

			getSDRCmd.Req.Offset = sdrHeaderLength
			getSDRCmd.Req.Length = header.Length
			if err := ValidateResponse(s.SendCommand(ctx, getSDRCmd)); err != nil {
				return nil, err
			}
			fsrPacket := gopacket.NewPacket(getSDRCmd.Rsp.Payload, ipmi.LayerTypeFullSensorRecord,
				gopacket.DecodeOptions{Lazy: true})
			if fsrPacket == nil {
				return nil, fmt.Errorf("invalid Full Sensor Record: %v", getSDRCmd)
			}
			fsrLayer := fsrPacket.Layer(ipmi.LayerTypeFullSensorRecord)
			if fsrLayer == nil {
				return nil, fmt.Errorf("packet is missing Full Sensor Record layer: %v",
					getSDRCmd)
			}
			repo[getSDRCmd.Req.RecordID] = fsrLayer.(*ipmi.FullSensorRecord)
		}

		getSDRCmd.Req.RecordID = getSDRCmd.Rsp.Next
		getSDRCmd.Req.Offset = 0x00
		getSDRCmd.Req.Length = sdrHeaderLength
	}
	return repo, nil
}
