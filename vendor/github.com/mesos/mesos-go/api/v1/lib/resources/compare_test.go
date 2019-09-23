package resources_test

import (
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"
	rez "github.com/mesos/mesos-go/api/v1/lib/resources"
	. "github.com/mesos/mesos-go/api/v1/lib/resourcetest"
)

func TestResources_ContainsAll(t *testing.T) {
	var (
		ports1 = Resources(Resource(Name("ports"), ValueRange(Span(2, 2), Span(4, 5)), Role("*")))
		ports2 = Resources(Resource(Name("ports"), ValueRange(Span(1, 10)), Role("*")))
		ports3 = Resources(Resource(Name("ports"), ValueRange(Span(2, 3)), Role("*")))
		ports4 = Resources(Resource(Name("ports"), ValueRange(Span(1, 2), Span(4, 6)), Role("*")))
		ports5 = Resources(Resource(Name("ports"), ValueRange(Span(1, 4), Span(5, 5)), Role("*")))

		disks1 = Resources(Resource(Name("disks"), ValueSet("sda1", "sda2"), Role("*")))
		disks2 = Resources(Resource(Name("disks"), ValueSet("sda1", "sda3", "sda4", "sda2"), Role("*")))

		disks = mesos.Resources{
			Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("1", "path")),
			Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("2", "path")),
			Resource(Name("disk"), ValueScalar(20), Role("role"), Disk("1", "path")),
			Resource(Name("disk"), ValueScalar(20), Role("role"), Disk("", "path")),
			Resource(Name("disk"), ValueScalar(20), Role("role"), Disk("2", "path")),
		}
		summedDisks  = Resources(disks[0]).Plus(disks[1])
		summedDisks2 = Resources(disks[0]).Plus(disks[4])

		revocables = mesos.Resources{
			Resource(Name("cpus"), ValueScalar(1), Role("*"), Revocable()),
			Resource(Name("cpus"), ValueScalar(1), Role("*")),
			Resource(Name("cpus"), ValueScalar(2), Role("*")),
			Resource(Name("cpus"), ValueScalar(2), Role("*"), Revocable()),
		}
		summedRevocables  = Resources(revocables[0]).Plus(revocables[1])
		summedRevocables2 = Resources(revocables[0]).Plus(revocables[0])

		// pre-refinement
		possiblyReserved = mesos.Resources{
			Resource(Name("cpus"), ValueScalar(8), Role("role")),
			Resource(Name("cpus"), ValueScalar(12), Role("role"), Reservation(ReservedBy("principal"))),
		}
		sumPossiblyReserved = Resources(possiblyReserved...)

		// refinement, static
		possiblyReserved2 = mesos.Resources{
			Resource(Name("cpus"), ValueScalar(8), Reservations(StaticReservation("role", ""))),
			Resource(Name("cpus"), ValueScalar(12), Reservations(StaticReservation("role", "principal"))),
		}
		sumPossiblyReserved2 = Resources(possiblyReserved2...)

		// refinement, dynamic
		possiblyReserved3 = mesos.Resources{
			Resource(Name("cpus"), ValueScalar(8), Reservations(DynamicReservation("role", ""))),
			Resource(Name("cpus"), ValueScalar(12), Reservations(DynamicReservation("role", "principal"))),
		}
		sumPossiblyReserved3 = Resources(possiblyReserved3...)

		// TODO: dynamic w/ labels
	)
	for i, tc := range []struct {
		r1, r2 mesos.Resources
		wants  bool
	}{
		// test case 0
		{r1: nil, r2: nil, wants: true},
		// test case 1
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(4096), Role("*")),
			),
			r2: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(4096), Role("*")),
			),
			wants: true,
		},
		// test case 2
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("role1")),
			),
			r2: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("role2")),
			),
			wants: false,
		},
		// test case 3
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(3072), Role("*")),
			),
			r2: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(4096), Role("*")),
			),
			wants: false,
		},
		// test case 4
		{
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(4096), Role("*")),
			),
			r2: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(3072), Role("*")),
			),
			wants: true,
		},
		// test case 5
		{ports2, ports1, true},
		// test case 6
		{ports1, ports2, false},
		// test case 7
		{ports3, ports1, false},
		// test case 8
		{ports1, ports3, false},
		// test case 9
		{ports2, ports3, true},
		// test case 10
		{ports3, ports2, false},
		// test case 11
		{ports4, ports1, true},
		// test case 12
		{ports2, ports4, true},
		// test case 13
		{ports5, ports1, true},
		// test case 14
		{ports1, ports5, false},
		// test case 15
		{disks1, disks2, false},
		// test case 16
		{disks2, disks1, true},
		{r1: summedDisks, r2: Resources(disks[0]), wants: true},
		{r1: summedDisks, r2: Resources(disks[1]), wants: true},
		{r1: summedDisks, r2: Resources(disks[2]), wants: false},
		{r1: summedDisks, r2: Resources(disks[3]), wants: false},
		{r1: Resources(disks[0]), r2: summedDisks, wants: false},
		{r1: Resources(disks[1]), r2: summedDisks, wants: false},
		{r1: summedDisks2, r2: Resources(disks[0]), wants: true},
		{r1: summedDisks2, r2: Resources(disks[4]), wants: true},
		{r1: summedRevocables, r2: Resources(revocables[0]), wants: true},
		{r1: summedRevocables, r2: Resources(revocables[1]), wants: true},
		{r1: summedRevocables, r2: Resources(revocables[2]), wants: false},
		{r1: summedRevocables, r2: Resources(revocables[3]), wants: false},
		{r1: Resources(revocables[0]), r2: summedRevocables2, wants: false},
		{r1: summedRevocables2, r2: Resources(revocables[0]), wants: true},
		{r1: summedRevocables2, r2: summedRevocables2, wants: true},
		{r1: Resources(possiblyReserved[0]), r2: sumPossiblyReserved, wants: false},
		{r1: Resources(possiblyReserved[1]), r2: sumPossiblyReserved, wants: false},

		{r1: sumPossiblyReserved, r2: Resources(possiblyReserved[0]), wants: true},
		{r1: sumPossiblyReserved, r2: Resources(possiblyReserved[1]), wants: true},
		{r1: sumPossiblyReserved, r2: sumPossiblyReserved, wants: true},

		{r1: sumPossiblyReserved2, r2: Resources(possiblyReserved2[0]), wants: true},
		{r1: sumPossiblyReserved2, r2: Resources(possiblyReserved2[1]), wants: true},
		{r1: sumPossiblyReserved2, r2: sumPossiblyReserved2, wants: true},

		{r1: sumPossiblyReserved3, r2: Resources(possiblyReserved3[0]), wants: true},
		{r1: sumPossiblyReserved3, r2: Resources(possiblyReserved3[1]), wants: true},
		{r1: sumPossiblyReserved3, r2: sumPossiblyReserved3, wants: true},
	} {
		actual := rez.ContainsAll(tc.r1, tc.r2)
		Expect(t, tc.wants == actual, "test case %d failed: wants (%v) != actual (%v)", i, tc.wants, actual)
	}
}

func TestResources_Validation(t *testing.T) {

	for ti, tc := range []struct {
		rs            []mesos.Resource
		wantsError    bool
		resourceError bool
	}{
		// don't use Resources(...) because that implicitly validates and skips invalid resources
		{
			mesos.Resources{
				Resource(Name("cpus"), ValueScalar(2), Role("*"), Disk("1", "path")),
			}, true, true, // expected resource error because cpu resources can't contain disk info
		},
		{rs: mesos.Resources{Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("1", "path"))}},
		{rs: mesos.Resources{Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("", "path"))}},
		// reservations
		{rs: mesos.Resources{Resource(Name("cpus"), ValueScalar(8), Role("*"))}},                                                // unreserved
		{rs: mesos.Resources{Resource(Name("cpus"), ValueScalar(8), Role("role"))}},                                             // statically reserved
		{rs: mesos.Resources{Resource(Name("cpus"), ValueScalar(8), Role("role"), Reservation(ReservedBy("principal2")))}},      // dynamically reserved
		{mesos.Resources{Resource(Name("cpus"), ValueScalar(8), Role("*"), Reservation(ReservedBy("principal1")))}, true, true}, // invalid reservation
	} {
		err := rez.Validate(tc.rs...)
		if tc.wantsError != (err != nil) {
			if tc.wantsError {
				t.Fatalf("test case %d failed: expected error", ti)
			} else {
				t.Fatalf("test case %d failed: unexpected error %+v", ti, err)
			}
		}
		if err != nil && tc.resourceError != mesos.IsResourceError(err) {
			if tc.resourceError {
				t.Fatalf("test case %d failed: expected resource error instead of error %+v", ti, err)
			} else {
				t.Fatalf("test case %d failed: unexpected resource error %+v", ti, err)
			}
		}
	}
}

func TestResources_Equivalent(t *testing.T) {
	disks := mesos.Resources{
		Resource(Name("disk"), ValueScalar(10), Role("*"), Disk("", "")),
		Resource(Name("disk"), ValueScalar(10), Role("*"), Disk("", "path1")),
		Resource(Name("disk"), ValueScalar(10), Role("*"), Disk("", "path2")),
		Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("", "path2")),
		Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("1", "path1")),
		Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("1", "path2")),
		Resource(Name("disk"), ValueScalar(10), Role("role"), Disk("2", "path2")),
		Resource(Name("disk"), ValueScalar(10), Role("*"), DiskWithSource("", "", "/mnt/path1", mesos.Resource_DiskInfo_Source_PATH)),
		Resource(Name("disk"), ValueScalar(10), Role("*"), DiskWithSource("", "", "/mnt/path2", mesos.Resource_DiskInfo_Source_PATH)),
		Resource(Name("disk"), ValueScalar(10), Role("*"), DiskWithSource("", "", "/mnt/path1", mesos.Resource_DiskInfo_Source_MOUNT)),
		Resource(Name("disk"), ValueScalar(10), Role("*"), DiskWithSource("", "", "/mnt/path2", mesos.Resource_DiskInfo_Source_MOUNT)),
	}
	for i, tc := range []struct {
		r1, r2 mesos.Resources
		wants  bool
	}{
		{r1: nil, r2: nil, wants: true},
		{ // 1
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(4096), Role("*")),
			),
			r2: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("*")),
				Resource(Name("mem"), ValueScalar(4096), Role("*")),
			),
			wants: true,
		},
		{ // 2
			r1: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("role1")),
			),
			r2: Resources(
				Resource(Name("cpus"), ValueScalar(50), Role("role2")),
			),
			wants: false,
		},
		{ // 3
			r1:    Resources(Resource(Name("ports"), ValueRange(Span(20, 40)), Role("*"))),
			r2:    Resources(Resource(Name("ports"), ValueRange(Span(20, 30), Span(31, 39), Span(40, 40)), Role("*"))),
			wants: true,
		},
		{ // 4
			r1:    Resources(Resource(Name("disks"), ValueSet("sda1"), Role("*"))),
			r2:    Resources(Resource(Name("disks"), ValueSet("sda1"), Role("*"))),
			wants: true,
		},
		{ // 5
			r1:    Resources(Resource(Name("disks"), ValueSet("sda1"), Role("*"))),
			r2:    Resources(Resource(Name("disks"), ValueSet("sda2"), Role("*"))),
			wants: false,
		},
		{Resources(disks[0]), Resources(disks[1]), true},   // 6
		{Resources(disks[1]), Resources(disks[2]), true},   // 7
		{Resources(disks[4]), Resources(disks[5]), true},   // 8
		{Resources(disks[5]), Resources(disks[6]), false},  // 9
		{Resources(disks[3]), Resources(disks[6]), false},  // 10
		{Resources(disks[0]), Resources(disks[7]), false},  // 11
		{Resources(disks[0]), Resources(disks[9]), false},  // 12
		{Resources(disks[7]), Resources(disks[9]), false},  // 13
		{Resources(disks[7]), Resources(disks[8]), false},  // 14
		{Resources(disks[9]), Resources(disks[10]), false}, // 15
		{ // 16
			r1:    Resources(Resource(Name("cpus"), ValueScalar(1), Role("*"), Revocable())),
			r2:    Resources(Resource(Name("cpus"), ValueScalar(1), Role("*"), Revocable())),
			wants: true,
		},
		{ // 17
			r1:    Resources(Resource(Name("cpus"), ValueScalar(1), Role("*"), Revocable())),
			r2:    Resources(Resource(Name("cpus"), ValueScalar(1), Role("*"))),
			wants: false,
		},
	} {
		actual := rez.Equivalent(tc.r1, tc.r2)
		Expect(t, tc.wants == actual, "test case %d failed: wants (%v) != actual (%v)", i, tc.wants, actual)
	}

	possiblyReserved := mesos.Resources{
		// unreserved
		Resource(Name("cpus"), ValueScalar(8), Role("*")),
		// statically role reserved
		Resource(Name("cpus"), ValueScalar(8), Role("role1")),
		Resource(Name("cpus"), ValueScalar(8), Role("role2")),
		// dynamically role reserved:
		Resource(Name("cpus"), ValueScalar(8), Role("role1"), Reservation(ReservedBy("principal1"))),
		Resource(Name("cpus"), ValueScalar(8), Role("role2"), Reservation(ReservedBy("principal2"))),
	}
	for i := 0; i < len(possiblyReserved); i++ {
		for j := 0; j < len(possiblyReserved); j++ {
			if i == j {
				continue
			}
			if rez.Equivalent(Resources(possiblyReserved[i]), Resources(possiblyReserved[j])) {
				t.Errorf("unexpected equivalence between %v and %v", possiblyReserved[i], possiblyReserved[j])
			}
		}
	}
}
