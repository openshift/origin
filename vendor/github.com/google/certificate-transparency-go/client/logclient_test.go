// Copyright 2014 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	ct "github.com/google/certificate-transparency-go"
	"github.com/google/certificate-transparency-go/client"
	"github.com/google/certificate-transparency-go/jsonclient"
	"github.com/google/certificate-transparency-go/testdata"
	"github.com/google/certificate-transparency-go/tls"
	"github.com/google/certificate-transparency-go/x509"
	"github.com/google/certificate-transparency-go/x509util"
)

func dh(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

const (
	ValidSTHResponse = `{"tree_size":3721782,"timestamp":1396609800587,
        "sha256_root_hash":"SxKOxksguvHPyUaKYKXoZHzXl91Q257+JQ0AUMlFfeo=",
        "tree_head_signature":"BAMARjBEAiBUYO2tODlUUw4oWGiVPUHqZadRRyXs9T2rSXchA79VsQIgLASkQv3cu4XdPFCZbgFkIUefniNPCpO3LzzHX53l+wg="}`
	ValidSTHResponseTreeSize          = 3721782
	ValidSTHResponseTimestamp         = 1396609800587
	ValidSTHResponseSHA256RootHash    = "SxKOxksguvHPyUaKYKXoZHzXl91Q257+JQ0AUMlFfeo="
	ValidSTHResponseTreeHeadSignature = "BAMARjBEAiBUYO2tODlUUw4oWGiVPUHqZadRRyXs9T2rSXchA79VsQIgLASkQv3cu4XdPFCZbgFkIUefniNPCpO3LzzHX53l+wg="

	PrecertEntryB64          = "AAAAAAFLSYHwyAABN2DieQ8zpJj5tsFJ/s/KOZOVS1NvvzatRdCoQVt5M30ABHowggR2oAMCAQICEAUyKYw5aj4l/KoZd+gntfMwDQYJKoZIhvcNAQELBQAwbTELMAkGA1UEBhMCVVMxFjAUBgNVBAoTDUdlb1RydXN0IEluYy4xHzAdBgNVBAsTFkZPUiBURVNUIFBVUlBPU0VTIE9OTFkxJTAjBgNVBAMTHEdlb1RydXN0IEVWIFNTTCBURVNUIENBIC0gRzQwHhcNMTUwMjAyMDAwMDAwWhcNMTYwMjI3MjM1OTU5WjCBwzETMBEGCysGAQQBgjc8AgEDEwJHQjEbMBkGCysGAQQBgjc8AgECFApDYWxpZm9ybmlhMR4wHAYLKwYBBAGCNzwCAQEMDU1vdW50YWluIFZpZXcxCzAJBgNVBAYTAkdCMRMwEQYDVQQIDApDYWxpZm9ybmlhMRYwFAYDVQQHDA1Nb3VudGFpbiBWaWV3MR0wGwYDVQQKDBRTeW1hbnRlYyBDb3Jwb3JhdGlvbjEWMBQGA1UEAwwNc2RmZWRzZi50cnVzdDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBALGdl97zn/gpxl6gmaMlcpizP/Z1RR/cVkGiIjR67kpWIB9MGkBvLxmBXYbewaYRdo59VWyOM6fxtMeNsZzOrlQOl64fBmCy7k+M/yBFuEqdoig0l0RAbs6u0LCNRv2rNUOz2G6nCGJ6YaUpt5Onatxrd2vI1bPU/iHixKqSz9M7RedBIGjgaDor7/rR3y/DILjdvwL/tgPSz3R5gnf9lla1rNRWWbDl12HgLc+VxTCVVVqTGtW/qbSWfARdXxLeLWtTfNk68q2LReVUC9QyeYdtE+N2+2SXeOEN+lYWW5Ab036d7k5GAntMBzLKftZEkYYquvaiSkqu2PSaCSLKT7UCAwEAAaOCAdEwggHNMEcGA1UdEQRAMD6CDWtqYXNkaGYudHJ1c3SCC3NzZGZzLnRydXN0gg1zZGZlZHNmLnRydXN0ghF3d3cuc2RmZWRzZi50cnVzdDAJBgNVHRMEAjAAMA4GA1UdDwEB/wQEAwIFoDArBgNVHR8EJDAiMCCgHqAchhpodHRwOi8vZ20uc3ltY2IuY29tL2dtLmNybDCBoAYDVR0gBIGYMIGVMIGSBgkrBgEEAfAiAQYwgYQwPwYIKwYBBQUHAgEWM2h0dHBzOi8vd3d3Lmdlb3RydXN0LmNvbS9yZXNvdXJjZXMvcmVwb3NpdG9yeS9sZWdhbDBBBggrBgEFBQcCAjA1DDNodHRwczovL3d3dy5nZW90cnVzdC5jb20vcmVzb3VyY2VzL3JlcG9zaXRvcnkvbGVnYWwwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsGAQUFBwMCMB8GA1UdIwQYMBaAFLFplGGr5ssMTOdZr1pJixgzweFHMFcGCCsGAQUFBwEBBEswSTAfBggrBgEFBQcwAYYTaHR0cDovL2dtLnN5bWNkLmNvbTAmBggrBgEFBQcwAoYaaHR0cDovL2dtLnN5bWNiLmNvbS9nbS5jcnQAAA=="
	PrecertEntryExtraDataB64 = "AAWnMIIFozCCBIugAwIBAgIQBTIpjDlqPiX8qhl36Ce18zANBgkqhkiG9w0BAQsFADBtMQswCQYDVQQGEwJVUzEWMBQGA1UEChMNR2VvVHJ1c3QgSW5jLjEfMB0GA1UECxMWRk9SIFRFU1QgUFVSUE9TRVMgT05MWTElMCMGA1UEAxMcR2VvVHJ1c3QgRVYgU1NMIFRFU1QgQ0EgLSBHNDAeFw0xNTAyMDIwMDAwMDBaFw0xNjAyMjcyMzU5NTlaMIHDMRMwEQYLKwYBBAGCNzwCAQMTAkdCMRswGQYLKwYBBAGCNzwCAQIUCkNhbGlmb3JuaWExHjAcBgsrBgEEAYI3PAIBAQwNTW91bnRhaW4gVmlldzELMAkGA1UEBhMCR0IxEzARBgNVBAgMCkNhbGlmb3JuaWExFjAUBgNVBAcMDU1vdW50YWluIFZpZXcxHTAbBgNVBAoMFFN5bWFudGVjIENvcnBvcmF0aW9uMRYwFAYDVQQDDA1zZGZlZHNmLnRydXN0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsZ2X3vOf+CnGXqCZoyVymLM/9nVFH9xWQaIiNHruSlYgH0waQG8vGYFdht7BphF2jn1VbI4zp/G0x42xnM6uVA6Xrh8GYLLuT4z/IEW4Sp2iKDSXREBuzq7QsI1G/as1Q7PYbqcIYnphpSm3k6dq3Gt3a8jVs9T+IeLEqpLP0ztF50EgaOBoOivv+tHfL8MguN2/Av+2A9LPdHmCd/2WVrWs1FZZsOXXYeAtz5XFMJVVWpMa1b+ptJZ8BF1fEt4ta1N82TryrYtF5VQL1DJ5h20T43b7ZJd44Q36VhZbkBvTfp3uTkYCe0wHMsp+1kSRhiq69qJKSq7Y9JoJIspPtQIDAQABo4IB5jCCAeIwRwYDVR0RBEAwPoINa2phc2RoZi50cnVzdIILc3NkZnMudHJ1c3SCDXNkZmVkc2YudHJ1c3SCEXd3dy5zZGZlZHNmLnRydXN0MAkGA1UdEwQCMAAwDgYDVR0PAQH/BAQDAgWgMCsGA1UdHwQkMCIwIKAeoByGGmh0dHA6Ly9nbS5zeW1jYi5jb20vZ20uY3JsMIGgBgNVHSAEgZgwgZUwgZIGCSsGAQQB8CIBBjCBhDA/BggrBgEFBQcCARYzaHR0cHM6Ly93d3cuZ2VvdHJ1c3QuY29tL3Jlc291cmNlcy9yZXBvc2l0b3J5L2xlZ2FsMEEGCCsGAQUFBwICMDUMM2h0dHBzOi8vd3d3Lmdlb3RydXN0LmNvbS9yZXNvdXJjZXMvcmVwb3NpdG9yeS9sZWdhbDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwHwYDVR0jBBgwFoAUsWmUYavmywxM51mvWkmLGDPB4UcwVwYIKwYBBQUHAQEESzBJMB8GCCsGAQUFBzABhhNodHRwOi8vZ20uc3ltY2QuY29tMCYGCCsGAQUFBzAChhpodHRwOi8vZ20uc3ltY2IuY29tL2dtLmNydDATBgorBgEEAdZ5AgQDAQH/BAIFADANBgkqhkiG9w0BAQsFAAOCAQEAZZrAobsW5UGOclcvWS0HmhnZKYz8TbLFIdrndp2+cwfETMmKOxoj8L6p50kMHMImKoS7udomH0TH0VKxuN9AnLYyo0MWehMpqHtICONUoCRXWaSOCp4I3Qz9bLgQZzw4nXrCfTscl/NmYh+U3VLYa5YiqbtzLeoDMg2pXQ3/f8z+9g4sR+6+tjYFSSfhvtZSFIHMJN8vCCHZIftNa2NxnFsQvp8V5GYCnZQOrbeqtoCgiZKeSnvDSEIPB+HQF4tHJTSiOd9I4DChzPn89vakUUWeiL4Z4ywf0ap7+1uFc5AlsUusm/VScsaGgJ9WPJqNtFXnnTkDpreL7gC+KetmJQAK7gAEKjCCBCYwggMOoAMCAQICEAPdhZRnA2PpjT8E8BujKHMwDQYJKoZIhvcNAQELBQAwfjELMAkGA1UEBhMCVVMxFjAUBgNVBAoTDUdlb1RydXN0IEluYy4xHzAdBgNVBAsTFkZPUiBURVNUIFBVUlBPU0VTIE9OTFkxNjA0BgNVBAMTLUdlb1RydXN0IFRFU1QgUHJpbWFyeSBDZXJ0aWZpY2F0aW9uIEF1dGhvcml0eTAeFw0xMzExMDEwMDAwMDBaFw0yMzEwMzEyMzU5NTlaMG0xCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1HZW9UcnVzdCBJbmMuMR8wHQYDVQQLExZGT1IgVEVTVCBQVVJQT1NFUyBPTkxZMSUwIwYDVQQDExxHZW9UcnVzdCBFViBTU0wgVEVTVCBDQSAtIEc0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsYDAA9HZOyqHErBSXabwfxlE3wUw13CTI64uJPKqLnxHs8Yeya2fueqzRy7U4vmuauj7ehlI/0hc7hpRPA5iwXzEB3H0H5m5KuYBQdbFGKu/JI09XYhFqhekjCbYkeuUk3AUAg16mIbihGvWXo/W09Rg6LPtrL/efQq4wDAEcY0PM/u5aqGRgT6Dg/SnWKEj9ZDTuRwbP8l0A9Vq3PR6Y1Gq8o+L+sIXA6qbt3wsl2A8Zavgi9pbCuVhJPf69I9FNs7cd20JwH30rou4Y+T4CNk4a5F00+QHDcyMhf92qP4+eCjWEtibyzUzeIt74KCUGWPwMbOtSYw7wBC7p4YqEwIDAQABo4GwMIGtMBIGA1UdEwEB/wQIMAYBAf8CAQAwDgYDVR0PAQH/BAQDAgEGMB0GA1UdDgQWBBSxaZRhq+bLDEznWa9aSYsYM8HhRzAfBgNVHSMEGDAWgBRuAQAD/r3QNjPgu2LhMlA2bZMdMTBHBgNVHSAEQDA+MDwGBFUdIAAwNDAyBggrBgEFBQcCARYmaHR0cHM6Ly93d3cuZ2VvdHJ1c3QuY29tL3Jlc291cmNlcy9jcHMwDQYJKoZIhvcNAQELBQADggEBAJk4Do4je+zfpGR7sLj/63Dtd5iUeJ1eNbLo3cmZesFXMCUSGhn6gfrKRmafjt6rXzpL/DHO7haLmSXGTAuq2xC1Up8tyUjNzyq25ShZEVJW3+ud7PW9W5I/ABKQaASmaDT/8tgGXkRzmfJd7lnV2y9F9KgcZ6fmOSe+EUwx/By7lVMCLrSAEVwpjQBA9pJjZ4V94ZVySu1AyeXqNkZ66Jcpc95ONlwIxmZn8gQe16tlo0rccxZ0ZjP7G3eK/1yyvFJW9yZRXw533HTfJWuVGzx40/CYNCVf97Oy51GEuULYHfNBtXFqGAiNHW2Rk+TdoUY/D89EgVHYGadbt9qbePoAA+0wggPpMIIDUqADAgECAhAgSXp/mbGJ2RfIkXHHcjw7MA0GCSqGSIb3DQEBBQUAMHQxCzAJBgNVBAYTAlVTMRAwDgYDVQQKEwdFcXVpZmF4MR8wHQYDVQQLExZGT1IgVEVTVCBQVVJQT1NFUyBPTkxZMTIwMAYDVQQLEylFcXVpZmF4IFNlY3VyZSBDZXJ0aWZpY2F0ZSBBdXRob3JpdHkgVEVTVDAeFw0wNjExMjcwMDAwMDBaFw0xODA4MjEyMzU5NTlaMH4xCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1HZW9UcnVzdCBJbmMuMR8wHQYDVQQLExZGT1IgVEVTVCBQVVJQT1NFUyBPTkxZMTYwNAYDVQQDEy1HZW9UcnVzdCBURVNUIFByaW1hcnkgQ2VydGlmaWNhdGlvbiBBdXRob3JpdHkwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDJFX7GBMMTPQ1ZFRc2D7SQebbPRk7+Zp5FcQ4Aj9dqaGtlK3TLwTw/1dcCbMjldXP3idcCvI45gXnAl3k5XqExRiaK0BE5wwvnt1Kv8xutRVTPfVwiyLbTRVRdLlL4DojklDnwXVEQAGx6sniBqJyf36dwb/4EHiNZBFe/kj0Ch4jihT4WeXv6b4e0qyL8dUGMtsEA+M0mU+Nc45aD3zAW2xDa6ZnBYdD//8PcUNNS3xp2ilGgIc03JgZj8mFNBONns8dHs8ZJGY9qxRiN0YwgqRhak185ixqXeJaWv7dKo8jAHmju+zz4nC2qNVTSHUe3HNO/14kAlPf3pzDBNzNtAgMBAAGjge0wgeowHQYDVR0OBBYEFG4BAAP+vdA2M+C7YuEyUDZtkx0xMA8GA1UdEwEB/wQFMAMBAf8wRgYDVR0gBD8wPTA7BgRVHSAAMDMwMQYIKwYBBQUHAgEWJWh0dHA6Ly93d3cuZ2VvdHJ1c3QuY29tL3Jlc291cmNlcy9jcHMwPwYDVR0fBDgwNjA0oDKgMIYuaHR0cDovL3Rlc3QtY3JsLmdlb3RydXN0LmNvbS9jcmxzL3NlY3VyZWNhLmNybDAOBgNVHQ8BAf8EBAMCAQYwHwYDVR0jBBgwFoAUiknD95Gj0KAaOuclQ8ExX0lIIGEwDQYJKoZIhvcNAQEFBQADgYEABOnm8HMpzjTS2sqXOv5bifuAzrqbrx0xzLaRi2xOW6hcb3Cx9KofdDyAepYWB0cRaV9eqqe1aHxaoMymH+1agaLufssE/jMLHNA9wAyZAr+DgAeaWzz0MWJJrRBEwhDnd/IMir/G3ydEs5NCLmak69wfXRGngoczPQSuMeN73FcAAs4wggLKMIICM6ADAgECAhAdn+8thA4gjKK8vUnAuDgzMA0GCSqGSIb3DQEBBQUAMHQxCzAJBgNVBAYTAlVTMRAwDgYDVQQKEwdFcXVpZmF4MR8wHQYDVQQLExZGT1IgVEVTVCBQVVJQT1NFUyBPTkxZMTIwMAYDVQQLEylFcXVpZmF4IFNlY3VyZSBDZXJ0aWZpY2F0ZSBBdXRob3JpdHkgVEVTVDAeFw05ODA4MjIwMDAwMDBaFw0xODA4MjIyMzU5NTlaMHQxCzAJBgNVBAYTAlVTMRAwDgYDVQQKEwdFcXVpZmF4MR8wHQYDVQQLExZGT1IgVEVTVCBQVVJQT1NFUyBPTkxZMTIwMAYDVQQLEylFcXVpZmF4IFNlY3VyZSBDZXJ0aWZpY2F0ZSBBdXRob3JpdHkgVEVTVDCBnzANBgkqhkiG9w0BAQEFAAOBjQAwgYkCgYEAmEaR7a3GQT5R12jnrMmZuFeWdcB9IYlxOfKQRyeEow/RT6vn5cd4ftBUYWaTsYT7tfalWmh3Q8PEExXpwDU6s4E/LOh7skQZteL5S0gq0a/zZeH2ugWQ3ZzQmyNg9sN7SHv7fdtqm3oCSqi/bsfhq6xVpIQjVYbHBogkHLwxrqUCAwEAAaNdMFswDAYDVR0TBAUwAwEB/zALBgNVHQ8EBAMCAQYwHQYDVR0OBBYEFIpJw/eRo9CgGjrnJUPBMV9JSCBhMB8GA1UdIwQYMBaAFIpJw/eRo9CgGjrnJUPBMV9JSCBhMA0GCSqGSIb3DQEBBQUAA4GBAHpWthUM2qNJ06MCtfpP1fkXw2IG0T6g69XOLcsPYaL51GmC3x84OpEGhLUycPtKDji3qSzu+Z5L+qkRjs5Sk6rIRIFex35oMn9EemprrkMUWIcOUnWSU25+XXr4Kmq4ItPS8FuUN5XT+coYqJ2UAW7QGaR11Gu7zf+6MrGgdJCk"

	CertEntryB64          = "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"
	CertEntryExtraDataB64 = "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"

	SubmissionCertB64 = "MIIEijCCA3KgAwIBAgICEk0wDQYJKoZIhvcNAQELBQAwKzEpMCcGA1UEAwwgY2Fja2xpbmcgY3J5cHRvZ3JhcGhlciBmYWtlIFJPT1QwHhcNMTUxMDIxMjAxMTUyWhcNMjAxMDE5MjAxMTUyWjAfMR0wGwYDVQQDExRoYXBweSBoYWNrZXIgZmFrZSBDQTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMIKR3maBcUSsncXYzQT13D5Nr+Z3mLxMMh3TUdt6sACmqbJ0btRlgXfMtNLM2OU1I6a3Ju+tIZSdn2v21JBwvxUzpZQ4zy2cimIiMQDZCQHJwzC9GZn8HaW091iz9H0Go3A7WDXwYNmsdLNRi00o14UjoaVqaPsYrZWvRKaIRqaU0hHmS0AWwQSvN/93iMIXuyiwywmkwKbWnnxCQ/gsctKFUtcNrwEx9Wgj6KlhwDTyI1QWSBbxVYNyUgPFzKxrSmwMO0yNff7ho+QT9x5+Y/7XE59S4Mc4ZXxcXKew/gSlN9U5mvT+D2BhDtkCupdfsZNCQWp27A+b/DmrFI9NqsCAwEAAaOCAcIwggG+MBIGA1UdEwEB/wQIMAYBAf8CAQAwQwYDVR0eBDwwOqE4MAaCBC5taWwwCocIAAAAAAAAAAAwIocgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAwDgYDVR0PAQH/BAQDAgGGMH8GCCsGAQUFBwEBBHMwcTAyBggrBgEFBQcwAYYmaHR0cDovL2lzcmcudHJ1c3RpZC5vY3NwLmlkZW50cnVzdC5jb20wOwYIKwYBBQUHMAKGL2h0dHA6Ly9hcHBzLmlkZW50cnVzdC5jb20vcm9vdHMvZHN0cm9vdGNheDMucDdjMB8GA1UdIwQYMBaAFOmkP+6epeby1dd5YDyTpi4kjpeqMFQGA1UdIARNMEswCAYGZ4EMAQIBMD8GCysGAQQBgt8TAQEBMDAwLgYIKwYBBQUHAgEWImh0dHA6Ly9jcHMucm9vdC14MS5sZXRzZW5jcnlwdC5vcmcwPAYDVR0fBDUwMzAxoC+gLYYraHR0cDovL2NybC5pZGVudHJ1c3QuY29tL0RTVFJPT1RDQVgzQ1JMLmNybDAdBgNVHQ4EFgQU+3hPEvlgFYMsnxd/NBmzLjbqQYkwDQYJKoZIhvcNAQELBQADggEBAA0YAeLXOklx4hhCikUUl+BdnFfn1g0W5AiQLVNIOL6PnqXu0wjnhNyhqdwnfhYMnoy4idRh4lB6pz8Gf9pnlLd/DnWSV3gS+/I/mAl1dCkKby6H2V790e6IHmIK2KYm3jm+U++FIdGpBdsQTSdmiX/rAyuxMDM0adMkNBwTfQmZQCz6nGHw1QcSPZMvZpsC8SkvekzxsjF1otOrMUPNPQvtTWrVx8GlR2qfx/4xbQa1v2frNvFBCmO59goz+jnWvfTtj2NjwDZ7vlMBsPm16dbKYC840uvRoZjxqsdc3ChCZjqimFqlNG/xoPA8+dTicZzCXE9ijPIcvW6y1aa3bGw="
	AddJSONResp       = `{
	   "sct_version":0,
	   "id":"KHYaGJAn++880NYaAY12sFBXKcenQRvMvfYE9F1CYVM=",
	   "timestamp":1337,
	   "extensions":"",
	   "signature":"BAMARjBEAiAIc21J5ZbdKZHw5wLxCP+MhBEsV5+nfvGyakOIv6FOvAIgWYMZb6Pw///uiNM7QTg2Of1OqmK1GbeGuEl9VJN8v8c="
	}`
	ProofByHashResp = `
	{
		"leaf_index": 3,
		"audit_path": [
		"pMumx96PIUB3TX543ljlpQ/RgZRqitRfykupIZrXq0Q=",
		"5s2NQWkjmesu+Kqgp70TCwVLwq8obpHw/JyMGwN56pQ=",
		"7VelXijfmGFSl62BWIsG8LRmxJGBq9XP8FxmszuT2Cg="
		]
	}`
	GetRootsResp = `
	{
		"certificates":[
		"MIIFLjCCAxagAwIBAgIQNgEiBHAkH6lLUWKp42Ob1DANBgkqhkiG9w0BAQ0FADAWMRQwEgYDVQQDEwtlc2lnbml0Lm9yZzAeFw0xNDA2MjAxODM3NTRaFw0zMDA2MjAxODQ3NDZaMBYxFDASBgNVBAMTC2VzaWduaXQub3JnMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAtylZx/zTLxRDsok14XO0Z3PvWMIY4HWro0YLgCF8dYv3tUaNkmN3ghlQvY8UcByH2LMOBGiQAcMHxgEJ53cnWRyc2DjoGhkDkiPdS2JttNEB0B/XTaGvaHwJh2CSgIBbpZpWTaqGywbe7AgJQ81L8h7tZ4E6W8ZM0vt4mnzqkPBT+BmyjTXG/McGhYTQAsmdsYZDBAdB2Y4X1/RAyL0e9MHdSboRofhg+8d5MeC0VEIgHXU/R4f4wz/pSw0FI9xxWJR3UUK/qOWqNsVYZfmCu6+ksDQtezxSTAuymoL094Dwn+hnXb8RS6dEbIQ+b0bIHxxpypcxH7rBMIpQcbZ8JSqNVDZPI9QahKNPQMQiuBE66KlqbnLOj7lGBxsbpU2Dx8QL8W96op6dTGtniFyXqhuYN2UxDMNI+fb1j9G7ENpoqvTVfjxa4RUU6uZ9ZygOiiOZD4P54vEQFteiu4OM+mWOm5Vll9yPXqHPc5oiCfyvCNVzfapqPoGbaCM6oQtcHdAca9VpE2eDTo36zfdFo31YYBOEjWNsfXwp8frNduS/L6gmWYrd91HeEoOVX2ZQKqBLp5ydW72xDSeCIr5kugqdY6whW80ugjLlc9mDd8/LEGQQKnrxzeeWdjiQG/WwcOse9GRktOzH2gvmkJ+vY82z1jhrZP4REoA6T+aYGR8CAwEAAaN4MHYwCwYDVR0PBAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFPOGsFKraD+/FoPAUXSf77qYfZHRMBIGCSsGAQQBgjcVAQQFAgMBAAEwIwYJKwYBBAGCNxUCBBYEFEq/BT//OC3eNeJ4wEfNqJXdZRNpMA0GCSqGSIb3DQEBDQUAA4ICAQBEvh2kzI+1uoUx/emM654QvpM6WtgQSJMubKwKeBY5UNgwwNpwmtswiEKzdZwBiGb1xEehPrAKz0d7aiIIEOonYEohIV6szl0+F56nN16813n1lPsCjdLSA8fjgf28jvlTKcrLRqeyCn4APadh6g7/FRiGcmIxEFPf/VNTUBZ7l4e2zzb06PxCq8oDaOsbAVYXQz8A0KX50KURZrdC2knUg1HX0J/orVpdaQ9UZYVNp2WAbe9vYTCCF5FdtzNU+nJDojpDxF5guMe9bifL3YTvd87YQwsH7+o+UbtHX4lG8VsSfmvvJulNBY6RtzZEpZvyRWIvQahM9qTrzFpsxl4wyPSBDPLDZ6YvVWsXvU4PqLOWTbPdq4BB24P9kFxeYjEe/rDQ8bd1/V/OFZTEM0rxdZDDN9vWnybzl8xL5VmNLDGl1u6JrOVvCzVAWP++L9l5UTusQI/BPSMebz6msd8vhTluD4jQIba1/6zOwfBraFgCIktCT3GEIiyt59x3rdSirLyjzmeQA9NkwoG/GqlFlSdWmQCK/sCL+z050rqjL0kEwIl/D6ncCXfBvhCpCmcrIlZFruyeOlsISZ410T1w/pLK8OXhbCr13Gb7A5jhv1nn811cQaR7XUXhcn6Wq/VV/oQZLunBYvoYOs3dc8wpBabPrrRhkdNmN6Rib6TvMg=="
		]
	}`
	GetSTHConsistencyResp = `{ "consistency": [ "IqlrapPQKtmCY1jCr8+lpCtscRyjjZAA7nyadtFPRFQ=", "ytf6K2GnSRZ3Au+YkivCb7N1DygfKyZmE4aEs9OXl\/8=" ] }`
	GetEntryAndProofResp  = `{
    "leaf_input": "AAAAAAFhw8UTtQAAAAJ1MIICcTCCAhegAwIBAgIFAN6tvu8wCgYIKoZIzj0EAwIwcjELMAkGA1UEBhMCR0IxDzANBgNVBAgTBkxvbmRvbjEPMA0GA1UEBxMGTG9uZG9uMQ8wDQYDVQQKEwZHb29nbGUxDDAKBgNVBAsTA0VuZzEiMCAGA1UEAxMZRmFrZUludGVybWVkaWF0ZUF1dGhvcml0eTAgFw0xNjEyMDcxNTEzMzZaGA8wMDAxMDEwMTAwMDAwMFowVjELMAkGA1UEBhMCR0IxDzANBgNVBAgMBkxvbmRvbjEPMA0GA1UECgwGR29vZ2xlMQwwCgYDVQQLDANFbmcxFzAVBgNVBAMMDmxlYWYwMS5jc3IucGVtMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE6zdOUkWcRtWouMXtWLkwKaZwimmgJlyeL264ayNshOFGOpg2gkSliheLQYIy9C3gCFt+BzhS/EdWKCeb7WCLrKOBszCBsDAPBgNVHQ8BAf8EBQMDB/mAMIGLBgNVHQ4EgYMEgYBPRBC+90lR8pRLbTi3ID4j0WRzjoJOT3MGkKko87o8z6gEifk9zCwOiHeIgclTA0ZUTxXMRI5r+nUY0frjRCWZu4uthPlE90iJM+RyjcNTwDJGu2StvLnJ8y4t5fdnwdGssncXiBQMuM7/1eMEwAOfHgTFzJ0UBC2Umztl0hul3zAPBgNVHSMECDAGgAQBAgMEMAoGCCqGSM49BAMCA0gAMEUCIQCrwywGKvyt/BwR+e7yDs78qt4sSEVJltv7Y0W6gOI5awIgQ+IAjejYivLEfqNufFRezCBWHWhbq/HHGdNQtv6EArkAAA==",
		"extra_data": "RXh0cmEK",
		"audit_path": [
		"pMumx96PIUB3TX543ljlpQ/RgZRqitRfykupIZrXq0Q=",
		"5s2NQWkjmesu+Kqgp70TCwVLwq8obpHw/JyMGwN56pQ=",
		"7VelXijfmGFSl62BWIsG8LRmxJGBq9XP8FxmszuT2Cg="
		]
  }`
)

func b64(s string) []byte {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

// serveHandlerAt returns a test HTTP server that only expects requests at the given path, and invokes
// the provided handler for that path.
func serveHandlerAt(t *testing.T, path string, handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == path {
			handler(w, r)
		} else {
			t.Fatalf("Incorrect URL path: %s", r.URL.Path)
		}
	}))
}

// serveRspAt returns a test HTTP server that returns a canned response body rsp for a given path.
func serveRspAt(t *testing.T, path, rsp string) *httptest.Server {
	t.Helper()
	return serveHandlerAt(t, path, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, rsp)
	})
}

func sctToJSON(rawSCT []byte) ([]byte, error) {
	var sct ct.SignedCertificateTimestamp
	_, err := tls.Unmarshal(rawSCT, &sct)
	if err != nil {
		return nil, fmt.Errorf("failed to tls-unmarshal test certificate proof: %v", err)
	}
	data, err := json.Marshal(sct)
	if err != nil {
		return nil, fmt.Errorf("failed to json-marshal test certificate proof: %v", err)
	}
	return data, nil
}

// serveSCTAt returns a test HTTP server that returns the given SCT as a canned response for
// a given path.
func serveSCTAt(t *testing.T, path string, rawSCT []byte) *httptest.Server {
	t.Helper()
	return serveHandlerAt(t, path, func(w http.ResponseWriter, r *http.Request) {
		data, err := sctToJSON(rawSCT)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(data)
	})
}

func TestGetEntries(t *testing.T) {
	ts := serveHandlerAt(t, "/ct/v1/get-entries", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		numRE := regexp.MustCompile("[0-9]+")
		if !numRE.MatchString(q["start"][0]) || !numRE.MatchString(q["end"][0]) {
			t.Fatalf("Invalid parameter: start=%q, end=%q", q["start"][0], q["end"][0])
		}
		fmt.Fprintf(w, `{"entries":[{"leaf_input": "%s","extra_data": "%s"},{"leaf_input": "%s","extra_data": "%s"}]}`,
			PrecertEntryB64,
			PrecertEntryExtraDataB64,
			CertEntryB64,
			CertEntryExtraDataB64)
	})
	defer ts.Close()
	lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	leaves, err := lc.GetEntries(context.Background(), 0, 1)
	if err != nil {
		t.Errorf("GetEntries(0,1)=nil,%v; want 2 leaves,nil", err)
	} else if len(leaves) != 2 {
		t.Errorf("GetEntries(0,1)=%d leaves,nil; want 2 leaves,nil", len(leaves))
	}
}

func TestGetEntriesErrors(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		start, end int64
		rsp, want  string
	}{
		{start: 1, end: 2, rsp: "", want: "EOF"},
		{start: 0, end: -1, want: "end should be >= 0"},
		{start: 3, end: 2, want: "start should be <= end"},
		{start: 4, end: 5, rsp: "not-json", want: "invalid"},
		{start: 5, end: 6, rsp: `{"entries":[{"leaf_input":"bogus","extra_data":"bogus"}]}`, want: "illegal base64"},
		{start: 6, end: 7, rsp: `{"entries":[{"leaf_input":"bbbb","extra_data":"bbbb"}]}`, want: "failed to unmarshal"},
	}

	for _, test := range tests {
		ts := serveRspAt(t, "/ct/v1/get-entries", test.rsp)
		defer ts.Close()
		lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Errorf("Failed to create client: %v", err)
			continue
		}
		got, err := lc.GetEntries(ctx, test.start, test.end)
		if err == nil {
			t.Errorf("GetEntries(%d, %d)=%+v, nil; want nil, %q", test.start, test.end, got, test.want)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("GetEntries(%d, %d)=nil, %q; want nil, %q", test.start, test.end, err, test.want)
		}
		if got != nil {
			t.Errorf("GetEntries(%d, %d)=%+v, _; want nil, _", test.start, test.end, got)
		}
	}
}

func TestGetRawEntriesErrors(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		start, end int64
		rsp, want  string
	}{
		{start: 1, end: 2, rsp: "", want: "EOF"},
		{start: 0, end: -1, want: "end should be >= 0"},
		{start: 3, end: 2, want: "start should be <= end"},
		{start: 4, end: 5, rsp: "not-json", want: "invalid"},
		{start: 5, end: 6, rsp: `{"entries":[{"leaf_input":"bogus","extra_data":"bogus"}]}`, want: "illegal base64"},
	}

	for _, test := range tests {
		ts := serveRspAt(t, "/ct/v1/get-entries", test.rsp)
		defer ts.Close()
		lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Errorf("Failed to create client: %v", err)
			continue
		}
		got, err := lc.GetRawEntries(ctx, test.start, test.end)
		if err == nil {
			t.Errorf("GetRawEntries(%d, %d)=%+v, nil; want nil, %q", test.start, test.end, got, test.want)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("GetRawEntries(%d, %d)=nil, %q; want nil, %q", test.start, test.end, err, test.want)
		}
		if got != nil {
			t.Errorf("GetRawEntries(%d, %d)=%+v, _; want nil, _", test.start, test.end, got)
		}
		if len(test.rsp) > 0 {
			// Expect the error to include the HTTP response
			if rspErr, ok := err.(client.RspError); !ok {
				t.Errorf("GetRawEntries(%d, %d)=nil, .(%T); want nil, .(RspError)", test.start, test.end, err)
			} else if string(rspErr.Body) != test.rsp {
				t.Errorf("GetRawEntries(%d, %d)=nil, .Body=%q; want nil, .Body=%q", test.start, test.end, rspErr.Body, test.rsp)
			}
		}
	}
}

func TestGetSTH(t *testing.T) {
	ts := serveRspAt(t, "/ct/v1/get-sth",
		fmt.Sprintf(`{"tree_size": %d, "timestamp": %d, "sha256_root_hash": "%s", "tree_head_signature": "%s"}`,
			ValidSTHResponseTreeSize,
			int64(ValidSTHResponseTimestamp),
			ValidSTHResponseSHA256RootHash,
			ValidSTHResponseTreeHeadSignature))
	defer ts.Close()
	lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	sth, err := lc.GetSTH(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if sth.TreeSize != ValidSTHResponseTreeSize {
		t.Errorf("GetSTH().TreeSize=%d; want %d", sth.TreeSize, ValidSTHResponseTreeSize)
	}
	if sth.Timestamp != ValidSTHResponseTimestamp {
		t.Errorf("GetSTH().Timestamp=%v; want %v", sth.Timestamp, ValidSTHResponseTimestamp)
	}
	if sth.SHA256RootHash.Base64String() != ValidSTHResponseSHA256RootHash {
		t.Errorf("GetSTH().SHA256RootHash=%v; want %v", sth.SHA256RootHash.Base64String(), ValidSTHResponseSHA256RootHash)
	}
	wantRawSignature, err := base64.StdEncoding.DecodeString(ValidSTHResponseTreeHeadSignature)
	if err != nil {
		t.Fatalf("Couldn't b64 decode 'correct' STH signature: %v", err)
	}
	var wantDS ct.DigitallySigned
	if _, err := tls.Unmarshal(wantRawSignature, &wantDS); err != nil {
		t.Fatalf("Couldn't unmarshal DigitallySigned: %v", err)
	}
	if sth.TreeHeadSignature.Algorithm.Hash != wantDS.Algorithm.Hash {
		t.Errorf("GetSTH().TreeHeadSignature.Algorithm.Hash=%v; %v", wantDS.Algorithm.Hash, sth.TreeHeadSignature.Algorithm.Hash)
	}
	if sth.TreeHeadSignature.Algorithm.Signature != wantDS.Algorithm.Signature {
		t.Errorf("GetSTH().TreeHeadSignature.Algorithm.Signature=%v; want %v", wantDS.Algorithm.Signature, sth.TreeHeadSignature.Algorithm.Signature)
	}
	if bytes.Compare(sth.TreeHeadSignature.Signature, wantDS.Signature) != 0 {
		t.Errorf("GetSTH().TreeHeadSignature.Signature=%v; want %v", wantDS.Signature, sth.TreeHeadSignature.Signature)
	}
}

func TestGetSTHErrors(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		rsp, want string
	}{
		{rsp: "", want: "EOF"},
		{rsp: "not-json", want: "invalid"},
		{rsp: `{"tree_size":228163,"timestamp":1507127718502,"sha256_root_hash":"bogus","tree_head_signature":"bogus"}`, want: "illegal base64"},
		{rsp: `{"tree_size":228163,"timestamp":1507127718502,"sha256_root_hash":"bbbb","tree_head_signature":"bbbb"}`, want: "hash is invalid length"},
		{rsp: `{"tree_size":228163,"timestamp":1507127718502,"sha256_root_hash":"tncuLXiPAo711IOxjaYTwLmwbSyyE8hEcRhaOXvFb3g=","tree_head_signature":"bbbb"}`, want: "syntax error"},
		{rsp: `{"tree_size":228163,"timestamp":1507127718502,"sha256_root_hash":"tncuLXiPAo711IOxjaYTwLmwbSyyE8hEcRhaOXvFb3g=","tree_head_signature":"BAMARjBEAiAi5045/h8Yvs1mNlsYskWvuFbu2A6hO2J45KDFfOR1OwIgZ2jq8iFCwKuTbcIgsBB1ibHEupv97CeAQynK0Dw2PT8bbbb="}`, want: "trailing data"},
	}

	for _, test := range tests {
		ts := serveRspAt(t, "/ct/v1/get-sth", test.rsp)
		defer ts.Close()
		lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Errorf("Failed to create client: %v", err)
			continue
		}
		got, err := lc.GetSTH(ctx)
		if err == nil {
			t.Errorf("GetSTH()=%+v, nil; want nil, %q", got, test.want)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("GetSTH()=nil, %q; want nil, %q", err, test.want)
		}
		if got != nil {
			t.Errorf("GetSTH()=%+v, _; want nil, _", got)
		}
		if len(test.rsp) > 0 {
			// Expect the error to include the HTTP response
			if rspErr, ok := err.(client.RspError); !ok {
				t.Errorf("GetSTH()=nil, .(%T); want nil, .(RspError)", err)
			} else if string(rspErr.Body) != test.rsp {
				t.Errorf("GetSTH()=nil, .Body=%q; want nil, .Body=%q", rspErr.Body, test.rsp)
			}
		}
	}
}

func TestAddChainRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping retry test in short mode")
	}
	retryAfter := 0 * time.Second
	currentFailures := 0
	failuresBeforeSuccess := 0
	hs := serveHandlerAt(t, "/ct/v1/add-chain", func(w http.ResponseWriter, r *http.Request) {
		if failuresBeforeSuccess > 0 && currentFailures < failuresBeforeSuccess {
			currentFailures++
			if retryAfter != 0 {
				if retryAfter > 0 {
					w.Header().Add("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				}
				w.WriteHeader(503)
				return
			}
			w.WriteHeader(408)
			return
		}
		_, err := w.Write([]byte(AddJSONResp))
		if err != nil {
			return
		}
	})
	defer hs.Close()

	certBytes, err := base64.StdEncoding.DecodeString(SubmissionCertB64)
	if err != nil {
		t.Fatalf("Failed to decode chain array B64: %s", err)
	}
	chain := []ct.ASN1Cert{{Data: certBytes}}

	const leeway = time.Millisecond * 100
	const leewayRatio = 0.2 // 20%

	tests := []struct {
		deadlineLength        time.Duration // -1 indicates no deadline
		expected              time.Duration
		retryAfter            time.Duration // -1 indicates: generate 503 with no Retry-After
		failuresBeforeSuccess int
		success               bool
	}{
		{
			deadlineLength:        -1,
			expected:              1 * time.Millisecond,
			retryAfter:            0,
			failuresBeforeSuccess: 0,
			success:               true,
		},
		{
			deadlineLength:        -1,
			expected:              7 * time.Second, // 1 + 2 + 4
			retryAfter:            -1,
			failuresBeforeSuccess: 3,
			success:               true,
		},
		{
			deadlineLength:        6 * time.Second,
			expected:              5 * time.Second,
			retryAfter:            5 * time.Second,
			failuresBeforeSuccess: 1,
			success:               true,
		},
		{
			deadlineLength:        5 * time.Second,
			expected:              5 * time.Second,
			retryAfter:            10 * time.Second,
			failuresBeforeSuccess: 1,
			success:               false,
		},
		{
			deadlineLength:        10 * time.Second,
			expected:              5 * time.Second,
			retryAfter:            1 * time.Second,
			failuresBeforeSuccess: 5,
			success:               true,
		},
		{
			deadlineLength:        1 * time.Second,
			expected:              10 * time.Millisecond,
			retryAfter:            0,
			failuresBeforeSuccess: 10,
			success:               true,
		},
	}

	for i, test := range tests {
		deadline := context.Background()
		lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		if test.deadlineLength >= 0 {
			var cancel context.CancelFunc
			deadline, cancel = context.WithDeadline(context.Background(), time.Now().Add(test.deadlineLength))
			defer cancel()
		}
		retryAfter = test.retryAfter
		failuresBeforeSuccess = test.failuresBeforeSuccess
		currentFailures = 0

		started := time.Now()
		sct, err := lc.AddChain(deadline, chain)
		took := time.Since(started)
		delta := math.Abs(float64(took - test.expected))
		ratio := delta / float64(test.expected)
		if delta > float64(leeway) && ratio > leewayRatio {
			t.Errorf("#%d Submission took an unexpected length of time: %s, expected ~%s", i, took, test.expected)
		}
		if test.success && err != nil {
			t.Errorf("#%d Failed to submit chain: %s", i, err)
		} else if !test.success && err == nil {
			t.Errorf("#%d Expected AddChain to fail", i)
		}
		if test.success && sct == nil {
			t.Errorf("#%d Nil SCT returned", i)
		}
	}
}

func TestAddChain(t *testing.T) {
	hs := serveSCTAt(t, "/ct/v1/add-chain", testdata.TestCertProof)
	defer hs.Close()
	lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{PublicKey: testdata.LogPublicKeyPEM})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	cert, err := x509util.CertificateFromPEM([]byte(testdata.TestCertPEM))
	if x509.IsFatal(err) {
		t.Fatalf("Failed to parse certificate from PEM: %v", err)
	}

	// AddChain will verify the signature because the client has a public key.
	chain := []ct.ASN1Cert{{Data: cert.Raw}}
	_, err = lc.AddChain(context.Background(), chain)
	if err != nil {
		t.Errorf("AddChain()=nil,%v; want sct,nil", err)
	}
}

func TestAddPreChain(t *testing.T) {
	hs := serveSCTAt(t, "/ct/v1/add-pre-chain", testdata.TestPreCertProof)
	defer hs.Close()
	lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{PublicKey: testdata.LogPublicKeyPEM})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	cert, err := x509util.CertificateFromPEM([]byte(testdata.TestPreCertPEM))
	if x509.IsFatal(err) {
		t.Fatalf("Failed to parse pre-certificate from PEM: %v", err)
	}
	issuer, err := x509util.CertificateFromPEM([]byte(testdata.CACertPEM))
	if x509.IsFatal(err) {
		t.Fatalf("Failed to parse issuer certificate from PEM: %v", err)
	}

	// AddPreChain will verify the signature because the client has a public key.
	chain := []ct.ASN1Cert{{Data: cert.Raw}, {Data: issuer.Raw}}
	_, err = lc.AddPreChain(context.Background(), chain)
	if err != nil {
		t.Errorf("AddPreChain()=nil,%v; want sct,nil", err)
	}
}

func TestAddJSON(t *testing.T) {
	hs := serveRspAt(t, "/ct/v1/add-json", AddJSONResp)
	defer hs.Close()
	lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		success bool
		data    interface{}
	}{
		{true, struct{ hi string }{"bob"}},
	}

	for _, test := range tests {
		sct, err := lc.AddJSON(context.Background(), test.data)
		if test.success && err != nil {
			t.Errorf("AddJSON(%v)=nil,%v; want sct,nil", test.data, err)
		} else if !test.success && err == nil {
			t.Errorf("AddJSON(%v)=sct,nil; want nil,error", test.data)
		}
		if test.success && sct == nil {
			t.Errorf("AddJSON(%v)=nil,%v; want sct,nil", test.data, err)
		}
	}
}

func TestGetSTHConsistency(t *testing.T) {
	hs := serveRspAt(t, "/ct/v1/get-sth-consistency", GetSTHConsistencyResp)
	defer hs.Close()
	lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		first  uint64
		second uint64
		proof  [][]byte
	}{
		{1, 3, [][]byte{
			b64("IqlrapPQKtmCY1jCr8+lpCtscRyjjZAA7nyadtFPRFQ="),
			b64("ytf6K2GnSRZ3Au+YkivCb7N1DygfKyZmE4aEs9OXl/8="),
		}},
	}

	for _, test := range tests {
		proof, err := lc.GetSTHConsistency(context.Background(), test.first, test.second)
		if err != nil {
			t.Errorf("GetSTHConsistency(%d, %d)=nil,%v; want proof,nil", test.first, test.second, err)
		} else if !reflect.DeepEqual(proof, test.proof) {
			t.Errorf("GetSTHConsistency(%d, %d)=%v,nil; want %v,nil", test.first, test.second, proof, test.proof)
		}
	}
}

func TestGetSTHConsistencyErrors(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		first, second uint64
		rsp, want     string
	}{
		{first: 1, second: 2, rsp: "", want: "EOF"},
		{first: 1, second: 2, rsp: "not-json", want: "invalid"},
		{first: 1, second: 2, rsp: `{"consistency":["bogus"]}`, want: "illegal base64"},
		{first: 1, second: 2, rsp: `{"consistency":["2SyPbmCNzn9l7dhWVz1uz6nW7DB7p0EkSsfH9M+qU5E=",]}`, want: "invalid"},
	}

	for _, test := range tests {
		ts := serveRspAt(t, "/ct/v1/get-sth-consistency", test.rsp)
		defer ts.Close()
		lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Errorf("Failed to create client: %v", err)
			continue
		}
		got, err := lc.GetSTHConsistency(ctx, test.first, test.second)
		if err == nil {
			t.Errorf("GetSTHConsistency(%d, %d)=%+v, nil; want nil, %q", test.first, test.second, got, test.want)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("GetSTHConsistency(%d, %d)=nil, %q; want nil, %q", test.first, test.second, err, test.want)
		}
		if got != nil {
			t.Errorf("GetSTHConsistency(%d, %d)=%+v, _; want nil, _", test.first, test.second, got)
		}
		if len(test.rsp) > 0 {
			// Expect the error to include the HTTP response
			if rspErr, ok := err.(client.RspError); !ok {
				t.Errorf("GetSTHConsistency(%d, %d)=nil, .(%T); want nil, .(RspError)", test.first, test.second, err)
			} else if string(rspErr.Body) != test.rsp {
				t.Errorf("GetSTHConsistency(%d, %d)=nil, .Body=%q; want nil, .Body=%q", test.first, test.second, rspErr.Body, test.rsp)
			}
		}
	}
}

func TestGetProofByHash(t *testing.T) {
	hs := serveRspAt(t, "/ct/v1/get-proof-by-hash", ProofByHashResp)
	defer hs.Close()
	lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		hash     []byte
		treesize uint64
	}{
		{dh("4a9e8edbe5ce2d2da69d483edb45186675d4be37b649d40923b156a7d1277463"), 5},
	}

	for _, test := range tests {
		resp, err := lc.GetProofByHash(context.Background(), test.hash, test.treesize)
		if err != nil {
			t.Errorf("GetProofByHash(%v, %v)=nil,%v; want proof,nil", test.hash, test.treesize, err)
		} else if got := len(resp.AuditPath); got < 1 {
			t.Errorf("len(GetProofByHash(%v, %v)): %v; want > 1", test.hash, test.treesize, got)
		}
	}
}

func TestGetProofByHashErrors(t *testing.T) {
	ctx := context.Background()
	aHash := dh("4a9e8edbe5ce2d2da69d483edb45186675d4be37b649d40923b156a7d1277463")
	var tests = []struct {
		rsp, want string
	}{
		{rsp: "", want: "EOF"},
		{rsp: "not-json", want: "invalid"},
		{rsp: `{"leaf_index": 17, "audit_path":["bogus"]}`, want: "illegal base64"},
		{rsp: `{"leaf_index": 17, "audit_path":["bbbb",]}`, want: "invalid"},
	}

	for _, test := range tests {
		ts := serveRspAt(t, "/ct/v1/get-proof-by-hash", test.rsp)
		defer ts.Close()
		lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Errorf("Failed to create client: %v", err)
			continue
		}
		got, err := lc.GetProofByHash(ctx, aHash, 100)
		if err == nil {
			t.Errorf("GetProofByHash()=%+v, nil; want nil, %q", got, test.want)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("GetProofByHash()=nil, %q; want nil, %q", err, test.want)
		}
		if got != nil {
			t.Errorf("GetProofByHash()=%+v, _; want nil, _", got)
		}
		if len(test.rsp) > 0 {
			// Expect the error to include the HTTP response
			if rspErr, ok := err.(client.RspError); !ok {
				t.Errorf("GetProofByHash()=nil, .(%T); want nil, .(RspError)", err)
			} else if string(rspErr.Body) != test.rsp {
				t.Errorf("GetProofByHash()=nil, .Body=%q; want nil, .Body=%q", rspErr.Body, test.rsp)
			}
		}
	}
}

func TestGetAcceptedRoots(t *testing.T) {
	hs := serveRspAt(t, "/ct/v1/get-roots", GetRootsResp)
	defer hs.Close()
	lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	certs, err := lc.GetAcceptedRoots(context.Background())
	if err != nil {
		t.Errorf("GetAcceptedRoots()=nil,%q; want roots,nil", err.Error())
	} else if len(certs) < 1 {
		t.Errorf("len(GetAcceptedRoots())=0; want > 1")
	}
}

func TestGetAcceptedRootsErrors(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		rsp, want string
	}{
		{rsp: "", want: "EOF"},
		{rsp: "not-json", want: "invalid"},
		{rsp: `{"certificates":["bogus"]}`, want: "illegal base64"},
		{rsp: `{"certificates":["bbbb",]}`, want: "invalid"},
	}

	for _, test := range tests {
		ts := serveRspAt(t, "/ct/v1/get-roots", test.rsp)
		defer ts.Close()
		lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Errorf("Failed to create client: %v", err)
			continue
		}
		got, err := lc.GetAcceptedRoots(ctx)
		if err == nil {
			t.Errorf("GetAcceptedRoots()=%+v, nil; want nil, %q", got, test.want)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("GetAcceptedRoots()=nil, %q; want nil, %q", err, test.want)
		}
		if got != nil {
			t.Errorf("GetAcceptedRoots()=%+v, _; want nil, _", got)
		}
		if len(test.rsp) > 0 {
			// Expect the error to include the HTTP response
			if rspErr, ok := err.(client.RspError); !ok {
				t.Errorf("GetAcceptedRoots()=nil, .(%T); want nil, .(RspError)", err)
			} else if string(rspErr.Body) != test.rsp {
				t.Errorf("GetAcceptedRoots()=nil, .Body=%q; want nil, .Body=%q", rspErr.Body, test.rsp)
			}
		}
	}
}

func TestGetEntryAndProof(t *testing.T) {
	hs := serveRspAt(t, "/ct/v1/get-entry-and-proof", GetEntryAndProofResp)
	defer hs.Close()
	lc, err := client.New(hs.URL, &http.Client{}, jsonclient.Options{})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		index    uint64
		treesize uint64
	}{
		{1000, 2000},
	}

	for _, test := range tests {
		resp, err := lc.GetEntryAndProof(context.Background(), test.index, test.treesize)
		if err != nil {
			t.Errorf("GetEntryAndProof(%v, %v)=nil,%v; want proof,nil", test.index, test.treesize, err)
		} else if got := len(resp.AuditPath); got < 1 {
			t.Errorf("len(GetEntryAndProof(%v, %v)): %v; want > 1", test.index, test.treesize, got)
		}
	}
}

func TestGetEntryAndProofErrors(t *testing.T) {
	ctx := context.Background()
	var tests = []struct {
		rsp, want string
	}{
		{rsp: "", want: "EOF"},
		{rsp: "not-json", want: "invalid"},
		{rsp: `{"leaf_input": "bogus", "extra_data": "Z29vZAo=", "audit_path": ["Z29vZAo="]}`, want: "illegal base64"},
		{rsp: `{"leaf_input": "Z29vZAo=", "extra_data": "bogus", "audit_path": ["Z29vZAo="]}`, want: "illegal base64"},
		{rsp: `{"leaf_input": "Z29vZAo=", "extra_data": "Z29vZAo=", "audit_path": ["bogus"]}`, want: "illegal base64"},
		{rsp: `{"leaf_input": "Z29vZAo=", "extra_data": "Z29vZAo=", "audit_path": ["bbbb",]}`, want: "invalid"},
	}

	for _, test := range tests {
		ts := serveRspAt(t, "/ct/v1/get-entry-and-proof", test.rsp)
		defer ts.Close()
		lc, err := client.New(ts.URL, &http.Client{}, jsonclient.Options{})
		if err != nil {
			t.Errorf("Failed to create client: %v", err)
			continue
		}
		got, err := lc.GetEntryAndProof(ctx, 99, 100)
		if err == nil {
			t.Errorf("GetEntryAndProof()=%+v, nil; want nil, %q", got, test.want)
		} else if !strings.Contains(err.Error(), test.want) {
			t.Errorf("GetEntryAndProof()=nil, %q; want nil, %q", err, test.want)
		}
		if got != nil {
			t.Errorf("GetEntryAndProof()=%+v, _; want nil, _", got)
		}
		if len(test.rsp) > 0 {
			// Expect the error to include the HTTP response
			if rspErr, ok := err.(client.RspError); !ok {
				t.Errorf("GetEntryAndProof()=nil, .(%T); want nil, .(RspError)", err)
			} else if string(rspErr.Body) != test.rsp {
				t.Errorf("GetEntryAndProof()=nil, .Body=%q; want nil, .Body=%q", rspErr.Body, test.rsp)
			}
		}
	}
}
