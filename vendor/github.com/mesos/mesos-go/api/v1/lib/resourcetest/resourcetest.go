package resourcetest

// maybe this could eventually be a resourcedsl package, but resource construction rules aren't that strict here yet

import (
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"
)

// Opt is a functional resource modifier
type Opt func(*mesos.Resource)

func Resource(opt ...Opt) (r mesos.Resource) {
	if len(opt) == 0 {
		return
	}
	for _, f := range opt {
		f(&r)
	}
	return
}

func Name(x string) Opt { return func(r *mesos.Resource) { r.Name = x } }
func Role(x string) Opt { return func(r *mesos.Resource) { r.Role = &x } }

func Revocable() Opt {
	return func(r *mesos.Resource) { r.Revocable = &mesos.Resource_RevocableInfo{} }
}

func ValueScalar(x float64) Opt {
	return func(r *mesos.Resource) {
		r.Type = mesos.SCALAR.Enum()
		r.Scalar = &mesos.Value_Scalar{Value: x}
	}
}

func ValueSet(x ...string) Opt {
	return func(r *mesos.Resource) {
		r.Type = mesos.SET.Enum()
		r.Set = &mesos.Value_Set{Item: x}
	}
}

type RangeOpt func(*mesos.Ranges)

// Span naively appends a range to a Ranges collection ("range" is a keyword, so I called this func "Span")
func Span(bp, ep uint64) RangeOpt {
	return func(rs *mesos.Ranges) {
		*rs = append(*rs, mesos.Value_Range{Begin: bp, End: ep})
	}
}

func ValueRange(p ...RangeOpt) Opt {
	return func(r *mesos.Resource) {
		rs := mesos.Ranges(nil)
		for _, f := range p {
			f(&rs)
		}
		r.Type = mesos.RANGES.Enum()
		r.Ranges = r.Ranges.Add(&mesos.Value_Ranges{Range: rs})
	}
}

func Resources(r ...mesos.Resource) (result mesos.Resources) {
	return result.Add(r...)
}

// Reservation should only be used for pre-reservation-refinement testing
func Reservation(ri *mesos.Resource_ReservationInfo) Opt {
	return func(r *mesos.Resource) {
		r.Reservation = ri
	}
}

// Reservations should only be used for post-reservation-refinement testing
func Reservations(ri ...mesos.Resource_ReservationInfo) Opt {
	return func(r *mesos.Resource) {
		r.Reservations = ri
	}
}

func Disk(persistenceID, containerPath string) Opt {
	return func(r *mesos.Resource) {
		r.Disk = &mesos.Resource_DiskInfo{}
		if containerPath != "" {
			r.Disk.Volume = &mesos.Volume{ContainerPath: containerPath}
		}
		if persistenceID != "" {
			r.Disk.Persistence = &mesos.Resource_DiskInfo_Persistence{ID: persistenceID}
		}
	}
}

func DiskWithSource(persistenceID, containerPath, source string, sourceType mesos.Resource_DiskInfo_Source_Type) Opt {
	return func(r *mesos.Resource) {
		r.Disk = &mesos.Resource_DiskInfo{}
		if containerPath != "" {
			r.Disk.Volume = &mesos.Volume{ContainerPath: containerPath}
		}
		if persistenceID != "" {
			r.Disk.Persistence = &mesos.Resource_DiskInfo_Persistence{ID: persistenceID}
		}
		if sourceType != mesos.Resource_DiskInfo_Source_UNKNOWN {
			r.Disk.Source = &mesos.Resource_DiskInfo_Source{Type: sourceType}
			switch sourceType {
			case mesos.Resource_DiskInfo_Source_PATH:
				if source != "" {
					r.Disk.Source.Path = &mesos.Resource_DiskInfo_Source_Path{Root: &source}
				}
			case mesos.Resource_DiskInfo_Source_MOUNT:
				if source != "" {
					r.Disk.Source.Mount = &mesos.Resource_DiskInfo_Source_Mount{Root: &source}
				}
			}
		}
	}
}

// ReservedBy returns a reservation for the given principal, if specified; intended for use with
// pre-reservation-refinement resources.
func ReservedBy(principal string) *mesos.Resource_ReservationInfo {
	result := &mesos.Resource_ReservationInfo{}
	if principal != "" {
		result.Principal = &principal
	}
	return result
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func makeReservation(t *mesos.Resource_ReservationInfo_Type, role, principal string, labels ...mesos.Label) (ri mesos.Resource_ReservationInfo) {
	ri.Type = t
	ri.Role = optionalString(role)
	ri.Principal = optionalString(principal)
	if len(labels) > 0 {
		ri.Labels = &mesos.Labels{Labels: labels}
	}
	return
}

func Label(k, v string) mesos.Label { return mesos.Label{Key: k, Value: optionalString(v)} }

func DynamicReservation(role, principal string, labels ...mesos.Label) (ri mesos.Resource_ReservationInfo) {
	ri = makeReservation(mesos.Resource_ReservationInfo_DYNAMIC.Enum(), role, principal, labels...)
	return
}

func StaticReservation(role, principal string, labels ...mesos.Label) (ri mesos.Resource_ReservationInfo) {
	ri = makeReservation(mesos.Resource_ReservationInfo_STATIC.Enum(), role, principal, labels...)
	return
}

func Reserve(r mesos.Resources) *mesos.Offer_Operation {
	return &mesos.Offer_Operation{
		Type: mesos.Offer_Operation_RESERVE,
		Reserve: &mesos.Offer_Operation_Reserve{
			Resources: r,
		},
	}
}

func Unreserve(r mesos.Resources) *mesos.Offer_Operation {
	return &mesos.Offer_Operation{
		Type: mesos.Offer_Operation_UNRESERVE,
		Unreserve: &mesos.Offer_Operation_Unreserve{
			Resources: r,
		},
	}
}

func Create(r mesos.Resources) *mesos.Offer_Operation {
	return &mesos.Offer_Operation{
		Type: mesos.Offer_Operation_CREATE,
		Create: &mesos.Offer_Operation_Create{
			Volumes: r,
		},
	}
}

func Expect(t *testing.T, cond bool, msgformat string, args ...interface{}) bool {
	if !cond {
		t.Errorf(msgformat, args...)
	}
	return cond
}
