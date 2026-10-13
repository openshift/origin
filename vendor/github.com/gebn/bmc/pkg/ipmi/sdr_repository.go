package ipmi

// RecordID uniquely identifies an SDR in the SDR Repository, and is used for
// access. Record IDs are not guaranteed to be enumerated sequentially, let
// alone consecutively, and there are likely to be gaps in them once the entire
// repository has been retrieved. It is more accurate in terms of the
// specification's guarantees to think of them as opaque keys into the SDR
// "map", rather than sparse array indices. The only requirement is that, at a
// given point in time (which is vague as IPMI does not have transactions), each
// SDR in the repo has a unique Record ID.
//
// A given Record ID is only valid until the SDR Repository is written to
// (either addition or removal of an SDR), at which point the controller may
// re-assign the ID to another SDR, or remove it entirely, giving the original
// SDR a new ID, possible in turn re-assigned from another SDR. This can be
// detected using the timestamps returned in "Get SDR Repository Info", however
// there is an inherent race condition here (the command does not support
// reservations), so instead we check the record key matches the one expected at
// retrieval time. See 33.8 in IPMI v2.0 for more detail. Note this is not the
// same as a sensor ID, which is a field within a sensor SDR (yes, sensor SDR -
// there are non-sensor SDRs e.g. Device Locator Records!).
type RecordID uint16

const (
	// RecordIDFirst always points to the "first" SDR in the repository. It is
	// used to kick off retrieval of SDRs by iterating through them.
	RecordIDFirst RecordID = 0x0000

	// RecordIDLast always points to the "last" SDR in this repository. The
	// "next" SDR will also be RecordIDLast. When this ID is seen, the record
	// should be read, then iteration stopped as the end has been reached.
	RecordIDLast RecordID = 0xffff
)

// ReservationID is a token returned by the BMC in response to the Reserve SDR
// Repository command, that is invalidated when SDR Record IDs may have changed.
// Reservations exist to solve the race problem where a record is identified for
// deletion or modification, but before it can be, the SDR is updated such that
// the Record ID changes, and the wrong SDR is updated. In addition to write
// operations, a reservation is required when reading from a non-0 offset of an
// SDR. Partial reads may be required even if the entire SDR is desired, when
// the SDR does not fit in the BMC's packet buffer.
//
// Note the SDR Repository device is allowed to not cancel the reservation if it
// knows a modification has not affected any existing Record IDs (33.11.2), so
// this cannot be used as a guarantee that the repository has not been changed
// or as a mechanism to watch for changes. It only guarantees consistency since
// the reservation was obtained. You need to occasionally check the timestamps
// returned by Get SDR Repository Info.
//
// Reserving the SDR repo effectively stamps it with an owner. The next
// application that attempts to reserve it will simply overwrite that value -
// there is no guarantee of access. Back-off, ideally with jitter, is essential
// to reservation logic, to ensure one of two competing applications will
// eventually be able to finish its work and they don't repeatedly stall each
// other. Unfortunately this relies on other applications doing the right thing.
type ReservationID uint16
