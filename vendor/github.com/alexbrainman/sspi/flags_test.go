package sspi

import (
	"strconv"
	"testing"
)

func Test_verifySelectiveFlags(t *testing.T) {
	type args struct {
		flags            uint32
		establishedFlags uint32
	}
	tests := []struct {
		name        string
		args        args
		wantValid   bool
		wantMissing uint32
		wantExtra   uint32
	}{
		{
			name: "all zeros",
			args: args{
				flags:            binary("00000"),
				establishedFlags: binary("00000"),
			},
			wantValid:   true,
			wantMissing: binary("00000"),
			wantExtra:   binary("00000"),
		},
		{
			name: "all ones",
			args: args{
				flags:            binary("11111"),
				establishedFlags: binary("11111"),
			},
			wantValid:   true,
			wantMissing: binary("00000"),
			wantExtra:   binary("00000"),
		},
		{
			name: "missing one bit",
			args: args{
				flags:            binary("11111"),
				establishedFlags: binary("11011"),
			},
			wantValid:   false,
			wantMissing: binary("00100"),
			wantExtra:   binary("00000"),
		},
		{
			name: "missing two bits",
			args: args{
				flags:            binary("11111"),
				establishedFlags: binary("01011"),
			},
			wantValid:   false,
			wantMissing: binary("10100"),
			wantExtra:   binary("00000"),
		},
		{
			name: "missing all bits",
			args: args{
				flags:            binary("11101"),
				establishedFlags: binary("00000"),
			},
			wantValid:   false,
			wantMissing: binary("11101"),
			wantExtra:   binary("00000"),
		},
		{
			name: "one extra bit",
			args: args{
				flags:            binary("00111"),
				establishedFlags: binary("01111"),
			},
			wantValid:   true,
			wantMissing: binary("00000"),
			wantExtra:   binary("01000"),
		},
		{
			name: "two extra bits",
			args: args{
				flags:            binary("01000"),
				establishedFlags: binary("11001"),
			},
			wantValid:   true,
			wantMissing: binary("00000"),
			wantExtra:   binary("10001"),
		},
		{
			name: "all extra bits",
			args: args{
				flags:            binary("00000"),
				establishedFlags: binary("11111"),
			},
			wantValid:   true,
			wantMissing: binary("00000"),
			wantExtra:   binary("11111"),
		},
		{
			name: "missing and extra bits",
			args: args{
				flags:            binary("00101"),
				establishedFlags: binary("11001"),
			},
			wantValid:   false,
			wantMissing: binary("00100"),
			wantExtra:   binary("11000"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValid, gotMissing, gotExtra := verifySelectiveFlags(tt.args.flags, tt.args.establishedFlags)
			if gotValid != tt.wantValid {
				t.Errorf("verifySelectiveFlags() gotValid = %v, want %v", gotValid, tt.wantValid)
			}
			if gotMissing != tt.wantMissing {
				t.Errorf("verifySelectiveFlags() gotMissing = %v, want %v", gotMissing, tt.wantMissing)
			}
			if gotExtra != tt.wantExtra {
				t.Errorf("verifySelectiveFlags() gotExtra = %v, want %v", gotExtra, tt.wantExtra)
			}
		})
	}
}

func binary(b string) uint32 {
	n, err := strconv.ParseUint(b, 2, 32)
	if err != nil {
		panic(err) // programmer error due to invalid test data
	}
	return uint32(n)
}
