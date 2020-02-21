package ebpf

import (
	"testing"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal/testutils"
)

func TestLink(t *testing.T) {
	spec := &ProgramSpec{
		Type: SocketFilter,
		Instructions: asm.Instructions{
			// Make sure the call doesn't happen at instruction 0
			// to exercise the relative offset calculation.
			asm.Mov.Reg(asm.R0, asm.R1),
			asm.Call.Label("my_func"),
			asm.Return(),
		},
		License: "MIT",
	}

	lib := &ProgramSpec{
		Instructions: asm.Instructions{
			asm.LoadImm(asm.R0, 1337, asm.DWord).Sym("my_func"),
			asm.Return(),
		},
	}

	err := link(spec, []*ProgramSpec{lib})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(spec.Instructions)

	testutils.SkipOnOldKernel(t, "4.16", "bpf2bpf calls")

	prog, err := NewProgram(spec)
	if err != nil {
		t.Fatal(err)
	}

	ret, _, err := prog.Test(make([]byte, 14))
	if err != nil {
		t.Fatal(err)
	}

	if ret != 1337 {
		t.Errorf("Expected return code 1337, got %d", ret)
	}
}
