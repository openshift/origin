package portutils

import (
	"strings"
	"testing"
)

type TestData struct {
	port  string
	ports []string
	err   string
	errs  []string
}

var (
	testData = []TestData{
		{
			port: "8080",
			err:  "",
		},
		{
			port: "8080/tcp",
			err:  "",
		},
		{
			port: "8080/udp",
			err:  "",
		},
		{
			port: "8080/TCP",
			err:  "",
		},
		{
			port: "8080/UDP",
			err:  "",
		},

		{
			port: "88080/tcp",
			err:  "port number must be in range 0 - 65535",
		},
		{
			port: "8080/xyz",
			err:  "protocol must be tcp or udp",
		},
		{
			port: "$myPort",
			err:  "port number must be in range 0 - 65535",
		},
	}

	testDataArray = []TestData{
		{
			ports: []string{"8080", "8080/tcp", "8080/udp"},
			errs:  []string{},
		},
		{
			ports: []string{"8080", "8080/TCP", "8080/UDP"},
			errs:  []string{},
		},
		{
			ports: []string{"8080", "$myPort/TCP", "8080/UDP"},
			errs:  []string{"port number must be in range 0 - 65535"},
		},

		{
			ports: []string{"8080", "88080/tcp", "8080/udp"},
			errs:  []string{"port number must be in range 0 - 65535"},
		},
		{
			ports: []string{"8080", "8080/xyz", "8080/udp"},
			errs:  []string{"protocol must be tcp or udp"},
		},
		{
			ports: []string{"88080", "8080/xyz", "8080/udp"},
			errs:  []string{"port number must be in range 0 - 65535", "protocol must be tcp or udp"},
		},
	}
)

func TestSplitPortAndProtocol(t *testing.T) {
	for _, data := range testData {
		dp, err := SplitPortAndProtocol(data.port)
		if data.err == "" && err != nil {
			t.Errorf("got error for %q but shouldn't have: %v", data.port, err)
		}
		if data.err != "" && err == nil {
			t.Errorf("should have gotten error for %q but didn't: %q", data.port, data.err)
		}
		if data.err != "" && err != nil {
			if !strings.Contains(err.Error(), data.err) {
				t.Errorf("returned incorrect error for %q: %v", data.port, err)
			}
		}
		portAndProto := strings.Split(data.port, "/")
		if portAndProto[0] != dp.Port() {
			t.Errorf("incorrect port %q returned, should have been %q", dp.Port(), portAndProto[0])
		}
		if len(portAndProto) == 2 && portAndProto[1] != dp.Proto() {
			t.Errorf("incorrect protocol %q returned, should have been %q", dp.Proto(), portAndProto[1])
		}
	}
}

func TestSplitPortAndProtocolArray(t *testing.T) {
	for _, data := range testDataArray {
		dps, errs := SplitPortAndProtocolArray(data.ports)
		if len(data.errs) == 0 && len(errs) != 0 {
			t.Errorf("got error for %q but shouldn't have: %v", data.ports, errs)
		}
		if len(data.errs) != 0 && len(errs) == 0 {
			t.Errorf("should have gotten error for %q but didn't: %v", data.ports, data.errs)
		}
		if len(data.errs) != 0 && len(errs) != 0 {
			var errorStrings []string
			for _, err := range errs {
				errorStrings = append(errorStrings, err.Error())
			}
			for _, err := range data.errs {
				if !strings.Contains(strings.Join(errorStrings, " "), err) {
					t.Errorf("returned incorrect error for %q: %v", data.ports, data.errs)
				}
			}
		}
		for i, dp := range dps {
			portAndProto := strings.Split(data.ports[i], "/")
			if portAndProto[0] != dp.Port() {
				t.Errorf("incorrect port %q returned, should have been %q", dp.Port(), portAndProto[0])
			}
			if len(portAndProto) == 2 && portAndProto[1] != dp.Proto() {
				t.Errorf("incorrect protocol %q returned, should have been %q", dp.Proto(), portAndProto[1])
			}
		}
	}
}
