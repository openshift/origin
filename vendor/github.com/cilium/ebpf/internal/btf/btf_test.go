package btf

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/cilium/ebpf/internal/testutils"
)

func TestParseVmlinux(t *testing.T) {
	fh, err := os.Open("testdata/vmlinux-btf.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	rd, err := gzip.NewReader(fh)
	if err != nil {
		t.Fatal(err)
	}

	buf, err := ioutil.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = parseBTF(bytes.NewReader(buf), binary.LittleEndian)
	if err != nil {
		t.Fatal("Can't load BTF:", err)
	}
}

func TestParseCurrentKernelBTF(t *testing.T) {
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); os.IsNotExist(err) {
		t.Skip("/sys/kernel/btf/vmlinux is not available")
	}

	fh, err := os.Open("/sys/kernel/btf/vmlinux")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	_, _, err = parseBTF(fh, binary.LittleEndian)
	if err != nil {
		t.Fatal("Can't load BTF:", err)
	}
}

func TestLoadSpecFromElf(t *testing.T) {
	fh, err := os.Open("../../testdata/loader-clang-9.elf")
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()

	spec, err := LoadSpecFromReader(fh)
	if err != nil {
		t.Fatal("Can't load BTF:", err)
	}

	if spec == nil {
		t.Error("No BTF found in ELF")
	}

	if sec, err := spec.Program("xdp", 1); err != nil {
		t.Error("Can't get BTF for the xdp section:", err)
	} else if sec == nil {
		t.Error("Missing BTF for the xdp section")
	}

	if sec, err := spec.Program("socket", 1); err != nil {
		t.Error("Can't get BTF for the socket section:", err)
	} else if sec == nil {
		t.Error("Missing BTF for the socket section")
	}

	var bpfMapDef Struct
	if err := spec.FindType("bpf_map_def", &bpfMapDef); err != nil {
		t.Fatal("Can't find bpf_map_def:", err)
	}

	if name := bpfMapDef.Name; name != "bpf_map_def" {
		t.Error("struct bpf_map_def has incorrect name:", name)
	}

	t.Run("Handle", func(t *testing.T) {
		btf, err := NewHandle(spec)
		testutils.SkipIfNotSupported(t, err)
		if err != nil {
			t.Fatal("Can't load BTF:", err)
		}
		defer btf.Close()
	})
}

func TestHaveBTF(t *testing.T) {
	testutils.CheckFeatureTest(t, haveBTF)
}

func ExampleSpec_FindType() {
	// Acquire a Spec via one of its constructors.
	spec := new(Spec)

	// Declare a variable of the desired type
	var foo Struct

	if err := spec.FindType("foo", &foo); err != nil {
		// There is no struct with name foo, or there
		// are multiple possibilities.
	}

	// We've found struct foo
	fmt.Println(foo.Name)
}
