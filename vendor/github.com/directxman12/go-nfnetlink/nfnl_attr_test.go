package nfnetlink

import (
	"testing"
)

func TestNewAttrFromFields(t *testing.T) {
	a, err := NewAttrFromFields(0, uint16(0xabcd))
	if err != nil {
		t.Errorf("Error creating attribute: %v", err)

	}
	check(t, "NewAttrFromFields", "06000000abcd0000", a.serialize())
}
