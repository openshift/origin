package asm

import (
	"testing"
)

func TestDSL(t *testing.T) {
	testcases := []struct {
		name string
		have Instruction
		want Instruction
	}{
		{"Call", FnMapLookupElem.Call(), Instruction{OpCode: 0x85, Constant: 1}},
		{"Exit", Return(), Instruction{OpCode: 0x95}},
		{"LoadAbs", LoadAbs(2, Byte), Instruction{OpCode: 0x30, Constant: 2}},
		{"Store", StoreMem(RFP, -4, R0, Word), Instruction{
			OpCode: 0x63,
			Dst:    RFP,
			Src:    R0,
			Offset: -4,
		}},
		{"Add.Imm", Add.Imm(R1, 22), Instruction{OpCode: 0x07, Dst: R1, Constant: 22}},
		{"Add.Reg", Add.Reg(R1, R2), Instruction{OpCode: 0x0f, Dst: R1, Src: R2}},
		{"Add.Imm32", Add.Imm32(R1, 22), Instruction{
			OpCode: 0x04, Dst: R1, Constant: 22,
		}},
	}

	for _, tc := range testcases {
		if tc.have != tc.want {
			t.Errorf("%s: have %v, want %v", tc.name, tc.have, tc.want)
		}
	}
}
