package ebpf

import (
	"testing"
)

func TestObjName(t *testing.T) {
	for in, valid := range map[string]bool{
		"test":                         true,
		"":                             true,
		"a-b":                          false,
		"yeah so":                      false,
		"more_than_16_characters_long": true,
	} {
		name, err := newBPFObjName(in)
		if result := err == nil; result != valid {
			t.Errorf("Name '%s' classified incorrectly", name)
		}
		if name[len(name)-1] != 0 {
			t.Errorf("Name '%s' is not null terminated", name)
		}
	}
}
