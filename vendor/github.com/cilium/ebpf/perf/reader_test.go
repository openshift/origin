package perf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal"
	"github.com/cilium/ebpf/internal/testutils"
	"github.com/cilium/ebpf/internal/unix"
)

var (
	readTimeout = 250 * time.Millisecond
)

func TestMain(m *testing.M) {
	err := unix.Setrlimit(8, &unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	})
	if err != nil {
		fmt.Println("WARNING: Failed to adjust rlimit, tests may fail")
	}
	os.Exit(m.Run())
}

func TestPerfReader(t *testing.T) {
	prog, events := mustOutputSamplesProg(t, 5)
	defer prog.Close()
	defer events.Close()

	rd, err := NewReader(events, 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	ret, _, err := prog.Test(make([]byte, 14))
	testutils.SkipIfNotSupported(t, err)
	if err != nil {
		t.Fatal(err)
	}

	if errno := syscall.Errno(-int32(ret)); errno != 0 {
		t.Fatal("Expected 0 as return value, got", errno)
	}

	record, err := rd.Read()
	if err != nil {
		t.Fatal("Can't read samples:", err)
	}

	want := []byte{1, 2, 3, 4, 4, 0, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(record.RawSample, want) {
		t.Log(record.RawSample)
		t.Error("Sample doesn't match expected output")
	}

	if record.CPU < 0 {
		t.Error("Record has invalid CPU number")
	}
}

func outputSamplesProg(sampleSizes ...int) (*ebpf.Program, *ebpf.Map, error) {
	const bpfFCurrentCPU = 0xffffffff

	events, err := ebpf.NewMap(&ebpf.MapSpec{
		Type: ebpf.PerfEventArray,
	})
	if err != nil {
		return nil, nil, err
	}

	var maxSampleSize int
	for _, sampleSize := range sampleSizes {
		if sampleSize > maxSampleSize {
			maxSampleSize = sampleSize
		}
	}

	// Fill a buffer on the stack, and stash context somewhere
	insns := asm.Instructions{
		asm.LoadImm(asm.R0, 0x0102030404030201, asm.DWord),
		asm.Mov.Reg(asm.R9, asm.R1),
	}

	bufDwords := (maxSampleSize / 8) + 1
	for i := 0; i < bufDwords; i++ {
		insns = append(insns,
			asm.StoreMem(asm.RFP, int16(i+1)*-8, asm.R0, asm.DWord),
		)
	}

	for _, sampleSize := range sampleSizes {
		insns = append(insns,
			asm.Mov.Reg(asm.R1, asm.R9),
			asm.LoadMapPtr(asm.R2, events.FD()),
			asm.LoadImm(asm.R3, bpfFCurrentCPU, asm.DWord),
			asm.Mov.Reg(asm.R4, asm.RFP),
			asm.Add.Imm(asm.R4, int32(bufDwords*-8)),
			asm.Mov.Imm(asm.R5, int32(sampleSize)),
			asm.FnPerfEventOutput.Call(),
		)
	}

	insns = append(insns, asm.Return())

	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		License:      "GPL",
		Type:         ebpf.XDP,
		Instructions: insns,
	})
	if err != nil {
		events.Close()
		return nil, nil, err
	}

	return prog, events, nil
}

func mustOutputSamplesProg(tb testing.TB, sampleSizes ...int) (*ebpf.Program, *ebpf.Map) {
	tb.Helper()

	prog, events, err := outputSamplesProg(sampleSizes...)
	if err != nil {
		tb.Fatal(err)
	}

	return prog, events
}

func TestPerfReaderLostSample(t *testing.T) {
	// To generate a lost sample perf record:
	//
	// 1. Fill the perf ring buffer almost completely, with the output_large program.
	//    The buffer is sized in number of pages, which are architecture dependant.
	//
	// 2. Write an extra event that doesn't fit in the space remaining.
	//
	// 3. Write a smaller event that does fit, with output_single program.
	//    Lost sample records are generated opportunistically, when the kernel
	//    is writing an event and realizes that there were events lost previously.
	//
	// The event size is hardcoded in the test BPF programs, there's no way
	// to parametrize it without rebuilding the programs.
	//
	// The event size needs to be selected so that, for any page size, there are at least
	// 48 bytes left in the perf ring page after filling it with a whole number of events:
	//
	//  - PERF_RECORD_LOST: 8 (perf_event_header) + 16 (PERF_RECORD_LOST)
	//
	//  - output_single: 8 (perf_event_header) + 4 (size) + 5 (payload) + 7 (padding to 64bits)
	//
	// By selecting an event size of the form 2^n + 2^(n+1), for any page size 2^(n+m), m >= 0,
	// the number of bytes left, x, after filling a page with a whole number of events is:
	//
	//                     2^(n+m)                            2^n * 2^m
	//  x = 2^n * frac(---------------) <=> x = 2^n * frac(---------------)
	//                  2^n + 2^(n+1)                       2^n + 2^n * 2
	//
	//                                                        2^n * 2^m
	//                                  <=> x = 2^n * frac(---------------)
	//                                                      2^n * (1 + 2)
	//
	//                                                      2^m
	//                                  <=> x = 2^n * frac(-----)
	//                                                       3
	//
	//                                                1                2
	//                                  <=> x = 2^n * -  or  x = 2^n * -
	//                                                3                3
	//
	// Selecting n = 6, we have:
	//
	//  x = 64  or  x = 128, no matter the page size 2^(6+m)
	//
	//  event size = 2^6 + 2^7 = 192
	//
	// Accounting for perf headers, output_large uses a 180 byte payload:
	//
	//  8 (perf_event_header) + 4 (size) + 180 (payload)
	const (
		eventSize = 192
	)

	var (
		pageSize  = os.Getpagesize()
		maxEvents = (pageSize / eventSize)
	)
	if remainder := pageSize % eventSize; remainder != 64 && remainder != 128 {
		// Page size isn't 2^(6+m), m >= 0
		t.Fatal("unsupported page size:", pageSize)
	}

	var sampleSizes []int
	// Fill the ring with the maximum number of output_large events that will fit,
	// and generate a lost event by writing an additional event.
	for i := 0; i < maxEvents+1; i++ {
		sampleSizes = append(sampleSizes, 180)
	}

	// Generate a small event to trigger the lost record
	sampleSizes = append(sampleSizes, 5)

	prog, events := mustOutputSamplesProg(t, sampleSizes...)
	defer prog.Close()
	defer events.Close()

	rd, err := NewReader(events, pageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	ret, _, err := prog.Test(make([]byte, 14))
	testutils.SkipIfNotSupported(t, err)
	if err != nil {
		t.Fatal(err)
	}

	if errno := syscall.Errno(-int32(ret)); errno != 0 {
		t.Fatal("Expected 0 as return value, got", errno)
	}

	for range sampleSizes {
		record, err := rd.Read()
		if err != nil {
			t.Fatal(err)
		}

		if record.RawSample == nil && record.LostSamples != 1 {
			t.Fatal("Expected a record with LostSamples 1, got", record.LostSamples)
		}
	}
}

func TestPerfReaderClose(t *testing.T) {
	prog, events := mustOutputSamplesProg(t, 5)
	defer prog.Close()
	defer events.Close()

	rd, err := NewReader(events, 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	errs := make(chan error, 1)
	waiting := make(chan struct{})
	go func() {
		close(waiting)
		_, err := rd.Read()
		errs <- err
	}()

	<-waiting

	// Close should interrupt Read
	if err := rd.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case <-errs:
	case <-time.After(time.Second):
		t.Fatal("Close doesn't interrupt Read")
	}

	// And we should be able to call it multiple times
	if err := rd.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := rd.Read(); err == nil {
		t.Fatal("Read on a closed PerfReader doesn't return an error")
	}
}

func TestCreatePerfEvent(t *testing.T) {
	fd, err := createPerfEvent(0, 1)
	if err != nil {
		t.Fatal("Can't create perf event:", err)
	}
	unix.Close(fd)
}

func TestReadRecord(t *testing.T) {
	var buf bytes.Buffer

	err := binary.Write(&buf, internal.NativeEndian, &perfEventHeader{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = readRecord(&buf, 0)
	if !IsUnknownEvent(err) {
		t.Error("readRecord should return unknown event error, got", err)
	}
}

func TestPause(t *testing.T) {
	t.Parallel()

	prog, events := mustOutputSamplesProg(t, 5)
	defer prog.Close()
	defer events.Close()

	rd, err := NewReader(events, 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()

	// Reader is already unpaused by default. It should be idempotent.
	if err = rd.Resume(); err != nil {
		t.Fatal(err)
	}

	// Write a sample. The reader should read it.
	ret, _, err := prog.Test(make([]byte, 14))
	testutils.SkipIfNotSupported(t, err)
	if err != nil || ret != 0 {
		t.Fatal("Can't write sample")
	}
	if _, err := rd.Read(); err != nil {
		t.Fatal(err)
	}

	// Pause. No notification should trigger.
	if err = rd.Pause(); err != nil {
		t.Fatal(err)
	}
	errChan := make(chan error, 1)
	go func() {
		// Read one notification then send any errors and exit.
		_, err := rd.Read()
		errChan <- err
	}()
	ret, _, err = prog.Test(make([]byte, 14))
	if err == nil && ret == 0 {
		t.Fatal("Unexpectedly wrote sample while paused")
	} // else Success
	select {
	case err := <-errChan:
		// Failure: Pause was unsuccessful.
		t.Fatalf("received notification on paused reader: %s", err)
	case <-time.After(readTimeout):
		// Success
	}

	// Pause should be idempotent.
	if err = rd.Pause(); err != nil {
		t.Fatal(err)
	}

	// Resume. Now notifications should continue.
	if err = rd.Resume(); err != nil {
		t.Fatal(err)
	}
	ret, _, err = prog.Test(make([]byte, 14))
	if err != nil || ret != 0 {
		t.Fatal("Can't write sample")
	}
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatal(err)
		} // else Success
	case <-time.After(readTimeout):
		t.Fatal("timed out waiting for notification after resume")
	}

	if err = rd.Close(); err != nil {
		t.Fatal(err)
	}

	// Pause/Resume after close should be no-op.
	if err = rd.Pause(); err != errClosed {
		t.Fatalf("Unexpected error: %s", err)
	}
	if err = rd.Resume(); err != errClosed {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func BenchmarkReader(b *testing.B) {
	prog, events := mustOutputSamplesProg(b, 80)
	defer prog.Close()
	defer events.Close()

	rd, err := NewReader(events, 4096)
	if err != nil {
		b.Fatal(err)
	}
	defer rd.Close()

	buf := make([]byte, 14)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ret, _, err := prog.Test(buf)
		if err != nil {
			b.Fatal(err)
		} else if errno := syscall.Errno(-int32(ret)); errno != 0 {
			b.Fatal("Expected 0 as return value, got", errno)
		}

		if _, err = rd.Read(); err != nil {
			b.Fatal(err)
		}
	}
}

// This exists just to make the example below nicer.
func bpfPerfEventOutputProgram() (*ebpf.Program, *ebpf.Map) {
	prog, events, err := outputSamplesProg(5)
	if err != nil {
		panic(err)
	}
	return prog, events
}

// ExamplePerfReader submits a perf event using BPF,
// and then reads it in user space.
//
// The BPF will look something like this:
//
//    struct map events __section("maps") = {
//      .type = BPF_MAP_TYPE_PERF_EVENT_ARRAY,
//    };
//
//    __section("xdp") int output_single(void *ctx) {
//      unsigned char buf[] = {
//        1, 2, 3, 4, 5
//      };
//
//       return perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &buf[0], 5);
//     }
//
// Also see BPF_F_CTXLEN_MASK if you want to sample packet data
// from SKB or XDP programs.
func ExampleReader() {
	prog, events := bpfPerfEventOutputProgram()
	defer prog.Close()
	defer events.Close()

	rd, err := NewReader(events, 4096)
	if err != nil {
		panic(err)
	}
	defer rd.Close()

	// Writes out a sample with content 1,2,3,4,4
	ret, _, err := prog.Test(make([]byte, 14))
	if err != nil || ret != 0 {
		panic("Can't write sample")
	}

	record, err := rd.Read()
	if err != nil {
		panic(err)
	}

	// Data is padded with 0 for alignment
	fmt.Println("Sample:", record.RawSample)
}
