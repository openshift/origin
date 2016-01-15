package util

import (
	"strings"
	"testing"
)

func TestCrypt(t *testing.T) {
	cases := map[string]struct {
		Key  string
		Salt string
		Out  string
	}{
		"crypt-simple": {
			Key:  "blah",
			Salt: "te",
			Out:  "te56PdudR.x9M",
		},
		"crypt-extra": {
			Key:  "blah",
			Salt: "testanoeutu",
			Out:  "te56PdudR.x9M",
		},
		"crypt-self": {
			Key:  "blah",
			Salt: "te56PdudR.x9M",
			Out:  "te56PdudR.x9M",
		},
		"modern-sha512": {
			Key:  "pass",
			Salt: "$6$zLlJ8bbRxoBEsHmj$",
			Out:  "$6$zLlJ8bbRxoBEsHmj$P2jDEu1tUP4aF2Nv5iRp1HUhNPsu/VKS6rDtC3k73G0Ijfx2m9qgZSMQI9aNDKDwqUuFSRhn5c0VLwyFRCxH7.",
		},
		"modern-self": {
			Key:  "pass",
			Salt: "$6$zLlJ8bbRxoBEsHmj$P2jDEu1tUP4aF2Nv5iRp1HUhNPsu/VKS6rDtC3k73G0Ijfx2m9qgZSMQI9aNDKDwqUuFSRhn5c0VLwyFRCxH7.",
			Out:  "$6$zLlJ8bbRxoBEsHmj$P2jDEu1tUP4aF2Nv5iRp1HUhNPsu/VKS6rDtC3k73G0Ijfx2m9qgZSMQI9aNDKDwqUuFSRhn5c0VLwyFRCxH7.",
		},
		"modern-md5": {
			Key:  "password",
			Salt: "$1$6VmuPCYl$",
			Out:  "$1$6VmuPCYl$tIEcT09v.tP3oo9YUAdUW/",
		},
		"modern-sha256": {
			Key:  "password",
			Salt: "$5$2034982094382039$",
			Out:  "$5$2034982094382039$NUCJMGCYb/mOLNYa5xwbpl4BqWy0lFev3.Lrs5GynI9",
		},
	}

	for k, testCase := range cases {
		out, err := Crypt(testCase.Key, testCase.Salt)
		if err != nil {
			// For crypt() on non-Linux platforms
			if !strings.Contains(err.Error(), "not supported on this platform") {
				t.Errorf("%s: Failed: %q", k, err)
			}
		} else if out != testCase.Out {
			t.Errorf("%s: Expected %s, got %s", k, testCase.Out, out)
		}
	}

	_, err := Crypt("blah", "")
	if err == nil {
		t.Errorf("crypt-no-salt: Should have failed")
	}
}
