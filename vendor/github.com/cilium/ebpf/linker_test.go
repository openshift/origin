package ebpf

import (
	"testing"

	"github.com/cilium/ebpf/asm"
)

func TestLink(t *testing.T) {
	insns := asm.Instructions{
		// Make sure the call doesn't happen at instruction 0
		// to exercise the relative offset calculation.
		asm.Mov.Reg(asm.R0, asm.R1),
		asm.Call.Label("my_func"),
		asm.Return(),
	}

	insns, err := link(insns, asm.Instructions{
		asm.LoadImm(asm.R0, 1337, asm.DWord).Sym("my_func"),
		asm.Return(),
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Log(insns)

	prog, err := NewProgram(&ProgramSpec{
		Type:         XDP,
		Instructions: insns,
		License:      "MIT",
	})
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
