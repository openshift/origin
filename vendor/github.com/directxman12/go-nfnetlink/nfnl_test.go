package nfnetlink

import (
	"encoding/hex"
	"testing"
)

func check(t *testing.T, testname string, expected string, data []byte) {
	encoded := hex.EncodeToString(data)
	if encoded != expected {
		t.Errorf("%s: expected %s got %s", testname, expected, encoded)
	}
}

func TestSerialize(t *testing.T) {
	msg := &NfNlMessage{}
	msg.Type = 0xAABB
	msg.Flags = 0xCCDD
	msg.Seq = 0xEEFFABCD
	msg.Pid = 0xCAFE1234
	msg.Family = 0x11
	msg.Version = 0x22
	msg.ResID = 0x3344
	check(t, "Serialize", "14000000bbaaddcccdabffee3412feca11223344", msg.Serialize())

}
