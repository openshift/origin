package ebpf

import (
	"strings"
	"testing"

	"github.com/cilium/ebpf/internal/testutils"
	"github.com/cilium/ebpf/internal/unix"
)

func TestObjNameCharacters(t *testing.T) {
	for in, valid := range map[string]bool{
		"test":    true,
		"":        true,
		"a-b":     false,
		"yeah so": false,
		"dot.":    objNameAllowsDot() == nil,
	} {
		result := strings.IndexFunc(in, invalidBPFObjNameChar) == -1
		if result != valid {
			t.Errorf("Name '%s' classified incorrectly", in)
		}
	}
}

func TestObjName(t *testing.T) {
	name := newBPFObjName("more_than_16_characters_long")
	if name[len(name)-1] != 0 {
		t.Error("newBPFObjName doesn't null terminate")
	}
	if len(name) != unix.BPF_OBJ_NAME_LEN {
		t.Errorf("Name is %d instead of %d bytes long", len(name), unix.BPF_OBJ_NAME_LEN)
	}
}

func TestHaveObjName(t *testing.T) {
	testutils.CheckFeatureTest(t, haveObjName)
}

func TestObjNameAllowsDot(t *testing.T) {
	testutils.CheckFeatureTest(t, objNameAllowsDot)
}

func TestHaveNestedMaps(t *testing.T) {
	testutils.CheckFeatureTest(t, haveNestedMaps)
}

func TestHaveMapMutabilityModifiers(t *testing.T) {
	testutils.CheckFeatureTest(t, haveMapMutabilityModifiers)
}
