// Copyright 2016 Google Inc. All Rights Reserved.
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

package testdata

import (
	"encoding/hex"
)

const (
	// CACertPEM is a CA cert:
	// Certificate:
	//     Data:
	//         Version: 3 (0x2)
	//         Serial Number: 0 (0x0)
	//     Signature Algorithm: sha1WithRSAEncryption
	//         Issuer: C=GB, O=Certificate Transparency CA, ST=Wales, L=Erw Wen
	//         Validity
	//             Not Before: Jun  1 00:00:00 2012 GMT
	//             Not After : Jun  1 00:00:00 2022 GMT
	//         Subject: C=GB, O=Certificate Transparency CA, ST=Wales, L=Erw Wen
	//         Subject Public Key Info:
	//             Public Key Algorithm: rsaEncryption
	//                 Public-Key: (1024 bit)
	//                 Modulus:
	//                     00:d5:8a:68:53:62:10:a2:71:19:93:6e:77:83:21:
	//                     18:1c:2a:40:13:c6:d0:7b:8c:76:eb:91:57:d3:d0:
	//                     fb:4b:3b:51:6e:ce:cb:d1:c9:8d:91:c5:2f:74:3f:
	//                     ab:63:5d:55:09:9c:d1:3a:ba:f3:1a:e5:41:44:24:
	//                     51:a7:4c:78:16:f2:24:3c:f8:48:cf:28:31:cc:e6:
	//                     7b:a0:4a:5a:23:81:9f:3c:ba:37:e6:24:d9:c3:bd:
	//                     b2:99:b8:39:dd:fe:26:31:d2:cb:3a:84:fc:7b:b2:
	//                     b5:c5:2f:cf:c1:4f:ff:40:6f:5c:d4:46:69:cb:b2:
	//                     f7:cf:df:86:fb:6a:b9:d1:b1
	//                 Exponent: 65537 (0x10001)
	//         X509v3 extensions:
	//             X509v3 Subject Key Identifier:
	//                 5F:9D:88:0D:C8:73:E6:54:D4:F8:0D:D8:E6:B0:C1:24:B4:47:C3:55
	//             X509v3 Authority Key Identifier:
	//                 keyid:5F:9D:88:0D:C8:73:E6:54:D4:F8:0D:D8:E6:B0:C1:24:B4:47:C3:55
	//                 DirName:/C=GB/O=Certificate Transparency CA/ST=Wales/L=Erw Wen
	//                 serial:00
	//
	//             X509v3 Basic Constraints:
	//                 CA:TRUE
	//     Signature Algorithm: sha1WithRSAEncryption
	//          06:08:cc:4a:6d:64:f2:20:5e:14:6c:04:b2:76:f9:2b:0e:fa:
	//          94:a5:da:f2:3a:fc:38:06:60:6d:39:90:d0:a1:ea:23:3d:40:
	//          29:57:69:46:3b:04:66:61:e7:fa:1d:17:99:15:20:9a:ea:2e:
	//          0a:77:51:76:41:12:27:d7:c0:03:07:c7:47:0e:61:58:4f:d7:
	//          33:42:24:72:7f:51:d6:90:bc:47:a9:df:35:4d:b0:f6:eb:25:
	//          95:5d:e1:89:3c:4d:d5:20:2b:24:a2:f3:e4:40:d2:74:b5:4e:
	//          1b:d3:76:26:9c:a9:62:89:b7:6e:ca:a4:10:90:e1:4f:3b:0a:
	//          94:2e
	CACertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIIC0DCCAjmgAwIBAgIBADANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFUxCzAJBgNVBAYTAkdCMSQwIgYDVQQKExtDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kgQ0ExDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGf\n" +
		"MA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDVimhTYhCicRmTbneDIRgcKkATxtB7\n" +
		"jHbrkVfT0PtLO1FuzsvRyY2RxS90P6tjXVUJnNE6uvMa5UFEJFGnTHgW8iQ8+EjP\n" +
		"KDHM5nugSlojgZ88ujfmJNnDvbKZuDnd/iYx0ss6hPx7srXFL8/BT/9Ab1zURmnL\n" +
		"svfP34b7arnRsQIDAQABo4GvMIGsMB0GA1UdDgQWBBRfnYgNyHPmVNT4DdjmsMEk\n" +
		"tEfDVTB9BgNVHSMEdjB0gBRfnYgNyHPmVNT4DdjmsMEktEfDVaFZpFcwVTELMAkG\n" +
		"A1UEBhMCR0IxJDAiBgNVBAoTG0NlcnRpZmljYXRlIFRyYW5zcGFyZW5jeSBDQTEO\n" +
		"MAwGA1UECBMFV2FsZXMxEDAOBgNVBAcTB0VydyBXZW6CAQAwDAYDVR0TBAUwAwEB\n" +
		"/zANBgkqhkiG9w0BAQUFAAOBgQAGCMxKbWTyIF4UbASydvkrDvqUpdryOvw4BmBt\n" +
		"OZDQoeojPUApV2lGOwRmYef6HReZFSCa6i4Kd1F2QRIn18ADB8dHDmFYT9czQiRy\n" +
		"f1HWkLxHqd81TbD26yWVXeGJPE3VICskovPkQNJ0tU4b03YmnKliibduyqQQkOFP\n" +
		"OwqULg==\n" +
		"-----END CERTIFICATE-----"

	// TestCertPEM is a leaf certificate signed by CACertPEM.
	// Certificate:
	//     Data:
	//         Version: 3 (0x2)
	//         Serial Number: 6 (0x6)
	//     Signature Algorithm: sha1WithRSAEncryption
	//         Issuer: C=GB, O=Certificate Transparency CA, ST=Wales, L=Erw Wen
	//         Validity
	//             Not Before: Jun  1 00:00:00 2012 GMT
	//             Not After : Jun  1 00:00:00 2022 GMT
	//         Subject: C=GB, O=Certificate Transparency, ST=Wales, L=Erw Wen
	//         Subject Public Key Info:
	//             Public Key Algorithm: rsaEncryption
	//                 Public-Key: (1024 bit)
	//                 Modulus:
	//                     00:b1:fa:37:93:61:11:f8:79:2d:a2:08:1c:3f:e4:
	//                     19:25:00:85:31:dc:7f:2c:65:7b:d9:e1:de:47:04:
	//                     16:0b:4c:9f:19:d5:4a:da:44:70:40:4c:1c:51:34:
	//                     1b:8f:1f:75:38:dd:dd:28:d9:ac:a4:83:69:fc:56:
	//                     46:dd:cc:76:17:f8:16:8a:ae:5b:41:d4:33:31:fc:
	//                     a2:da:df:c8:04:d5:72:08:94:90:61:f9:ee:f9:02:
	//                     ca:47:ce:88:c6:44:e0:00:f0:6e:ee:cc:ab:dc:9d:
	//                     d2:f6:8a:22:cc:b0:9d:c7:6e:0d:bc:73:52:77:65:
	//                     b1:a3:7a:8c:67:62:53:dc:c1
	//                 Exponent: 65537 (0x10001)
	//         X509v3 extensions:
	//             X509v3 Subject Key Identifier:
	//                 6A:0D:98:2A:3B:62:C4:4B:6D:2E:F4:E9:BB:7A:01:AA:9C:B7:98:E2
	//             X509v3 Authority Key Identifier:
	//                 keyid:5F:9D:88:0D:C8:73:E6:54:D4:F8:0D:D8:E6:B0:C1:24:B4:47:C3:55
	//                 DirName:/C=GB/O=Certificate Transparency CA/ST=Wales/L=Erw Wen
	//                 serial:00
	//
	//             X509v3 Basic Constraints:
	//                 CA:FALSE
	//     Signature Algorithm: sha1WithRSAEncryption
	//          17:1c:d8:4a:ac:41:4a:9a:03:0f:22:aa:c8:f6:88:b0:81:b2:
	//          70:9b:84:8b:4e:55:11:40:6c:d7:07:fe:d0:28:59:7a:9f:ae:
	//          fc:2e:ee:29:78:d6:33:aa:ac:14:ed:32:35:19:7d:a8:7e:0f:
	//          71:b8:87:5f:1a:c9:e7:8b:28:17:49:dd:ed:d0:07:e3:ec:f5:
	//          06:45:f8:cb:f6:67:25:6c:d6:a1:64:7b:5e:13:20:3b:b8:58:
	//          2d:e7:d6:69:6f:65:6d:1c:60:b9:5f:45:6b:7f:cf:33:85:71:
	//          90:8f:1c:69:72:7d:24:c4:fc:cd:24:92:95:79:58:14:d1:da:
	//          c0:e6
	TestCertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIICyjCCAjOgAwIBAgIBBjANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFIxCzAJBgNVBAYTAkdCMSEwHwYDVQQKExhDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kxDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGfMA0G\n" +
		"CSqGSIb3DQEBAQUAA4GNADCBiQKBgQCx+jeTYRH4eS2iCBw/5BklAIUx3H8sZXvZ\n" +
		"4d5HBBYLTJ8Z1UraRHBATBxRNBuPH3U43d0o2aykg2n8VkbdzHYX+BaKrltB1DMx\n" +
		"/KLa38gE1XIIlJBh+e75AspHzojGROAA8G7uzKvcndL2iiLMsJ3Hbg28c1J3ZbGj\n" +
		"eoxnYlPcwQIDAQABo4GsMIGpMB0GA1UdDgQWBBRqDZgqO2LES20u9Om7egGqnLeY\n" +
		"4jB9BgNVHSMEdjB0gBRfnYgNyHPmVNT4DdjmsMEktEfDVaFZpFcwVTELMAkGA1UE\n" +
		"BhMCR0IxJDAiBgNVBAoTG0NlcnRpZmljYXRlIFRyYW5zcGFyZW5jeSBDQTEOMAwG\n" +
		"A1UECBMFV2FsZXMxEDAOBgNVBAcTB0VydyBXZW6CAQAwCQYDVR0TBAIwADANBgkq\n" +
		"hkiG9w0BAQUFAAOBgQAXHNhKrEFKmgMPIqrI9oiwgbJwm4SLTlURQGzXB/7QKFl6\n" +
		"n678Lu4peNYzqqwU7TI1GX2ofg9xuIdfGsnniygXSd3t0Afj7PUGRfjL9mclbNah\n" +
		"ZHteEyA7uFgt59Zpb2VtHGC5X0Vrf88zhXGQjxxpcn0kxPzNJJKVeVgU0drA5g==\n" +
		"-----END CERTIFICATE-----\n"

	// TestPreCertPEM is a pre-certificate signed by CACertPEM.
	// Certificate:
	//     Data:
	//         Version: 3 (0x2)
	//         Serial Number: 7 (0x7)
	//     Signature Algorithm: sha1WithRSAEncryption
	//         Issuer: C=GB, O=Certificate Transparency CA, ST=Wales, L=Erw Wen
	//         Validity
	//             Not Before: Jun  1 00:00:00 2012 GMT
	//             Not After : Jun  1 00:00:00 2022 GMT
	//         Subject: C=GB, O=Certificate Transparency, ST=Wales, L=Erw Wen
	//         Subject Public Key Info:
	//             Public Key Algorithm: rsaEncryption
	//                 Public-Key: (1024 bit)
	//                 Modulus:
	//                     00:be:ef:98:e7:c2:68:77:ae:38:5f:75:32:5a:0c:
	//                     1d:32:9b:ed:f1:8f:aa:f4:d7:96:bf:04:7e:b7:e1:
	//                     ce:15:c9:5b:a2:f8:0e:e4:58:bd:7d:b8:6f:8a:4b:
	//                     25:21:91:a7:9b:d7:00:c3:8e:9c:03:89:b4:5c:d4:
	//                     dc:9a:12:0a:b2:1e:0c:b4:1c:d0:e7:28:05:a4:10:
	//                     cd:9c:5b:db:5d:49:27:72:6d:af:17:10:f6:01:87:
	//                     37:7e:a2:5b:1a:1e:39:ee:d0:b8:81:19:dc:15:4d:
	//                     c6:8f:7d:a8:e3:0c:af:15:8a:33:e6:c9:50:9f:4a:
	//                     05:b0:14:09:ff:5d:d8:7e:b5
	//                 Exponent: 65537 (0x10001)
	//         X509v3 extensions:
	//             X509v3 Subject Key Identifier:
	//                 20:31:54:1A:F2:5C:05:FF:D8:65:8B:68:43:79:4F:5E:90:36:F7:B4
	//             X509v3 Authority Key Identifier:
	//                 keyid:5F:9D:88:0D:C8:73:E6:54:D4:F8:0D:D8:E6:B0:C1:24:B4:47:C3:55
	//                 DirName:/C=GB/O=Certificate Transparency CA/ST=Wales/L=Erw Wen
	//                 serial:00
	//
	//             X509v3 Basic Constraints:
	//                 CA:FALSE
	//             1.3.6.1.4.1.11129.2.4.3: critical
	//                 ..
	//     Signature Algorithm: sha1WithRSAEncryption
	//          02:a1:c3:9e:01:5a:f5:4d:ff:02:3c:33:60:87:5f:ff:34:37:
	//          55:2f:1f:09:01:bd:c2:54:31:5f:33:72:b7:23:fb:15:fb:ce:
	//          cc:4d:f4:71:a0:ce:4d:8c:54:65:5d:84:87:97:fb:28:1e:3d:
	//          fa:bb:46:2d:2c:68:4b:05:6f:ea:7b:63:b4:70:ff:16:6e:32:
	//          d4:46:06:35:b3:d2:bc:6d:a8:24:9b:26:30:e7:1f:c3:4f:08:
	//          f2:3d:d4:ee:22:8f:8f:74:f6:3d:78:63:11:dd:0a:58:11:40:
	//          5f:90:6c:ca:2c:2d:3e:eb:fc:81:99:64:eb:d8:cf:7c:08:86:
	//          3f:be
	TestPreCertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIIC3zCCAkigAwIBAgIBBzANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFIxCzAJBgNVBAYTAkdCMSEwHwYDVQQKExhDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kxDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGfMA0G\n" +
		"CSqGSIb3DQEBAQUAA4GNADCBiQKBgQC+75jnwmh3rjhfdTJaDB0ym+3xj6r015a/\n" +
		"BH634c4VyVui+A7kWL19uG+KSyUhkaeb1wDDjpwDibRc1NyaEgqyHgy0HNDnKAWk\n" +
		"EM2cW9tdSSdyba8XEPYBhzd+olsaHjnu0LiBGdwVTcaPfajjDK8VijPmyVCfSgWw\n" +
		"FAn/Xdh+tQIDAQABo4HBMIG+MB0GA1UdDgQWBBQgMVQa8lwF/9hli2hDeU9ekDb3\n" +
		"tDB9BgNVHSMEdjB0gBRfnYgNyHPmVNT4DdjmsMEktEfDVaFZpFcwVTELMAkGA1UE\n" +
		"BhMCR0IxJDAiBgNVBAoTG0NlcnRpZmljYXRlIFRyYW5zcGFyZW5jeSBDQTEOMAwG\n" +
		"A1UECBMFV2FsZXMxEDAOBgNVBAcTB0VydyBXZW6CAQAwCQYDVR0TBAIwADATBgor\n" +
		"BgEEAdZ5AgQDAQH/BAIFADANBgkqhkiG9w0BAQUFAAOBgQACocOeAVr1Tf8CPDNg\n" +
		"h1//NDdVLx8JAb3CVDFfM3K3I/sV+87MTfRxoM5NjFRlXYSHl/soHj36u0YtLGhL\n" +
		"BW/qe2O0cP8WbjLURgY1s9K8bagkmyYw5x/DTwjyPdTuIo+PdPY9eGMR3QpYEUBf\n" +
		"kGzKLC0+6/yBmWTr2M98CIY/vg==\n" +
		"-----END CERTIFICATE-----\n"

	// TestEmbeddedCertPEM is a certificate containing an embedded SCT that
	// corresponds to TestPreCertProof.
	//	Certificate:
	//		Data:
	//			Version: 3 (0x2)
	//			Serial Number: 7 (0x7)
	//		Signature Algorithm: sha1WithRSAEncryption
	//			Issuer: C = GB, O = Certificate Transparency CA, ST = Wales, L = Erw Wen
	//			Validity
	//				Not Before: Jun  1 00:00:00 2012 GMT
	//				Not After : Jun  1 00:00:00 2022 GMT
	//			Subject: C = GB, O = Certificate Transparency, ST = Wales, L = Erw Wen
	//			Subject Public Key Info:
	//				Public Key Algorithm: rsaEncryption
	//					Public-Key: (1024 bit)
	//					Modulus:
	//						00:be:ef:98:e7:c2:68:77:ae:38:5f:75:32:5a:0c:
	//						1d:32:9b:ed:f1:8f:aa:f4:d7:96:bf:04:7e:b7:e1:
	//						ce:15:c9:5b:a2:f8:0e:e4:58:bd:7d:b8:6f:8a:4b:
	//						25:21:91:a7:9b:d7:00:c3:8e:9c:03:89:b4:5c:d4:
	//						dc:9a:12:0a:b2:1e:0c:b4:1c:d0:e7:28:05:a4:10:
	//						cd:9c:5b:db:5d:49:27:72:6d:af:17:10:f6:01:87:
	//						37:7e:a2:5b:1a:1e:39:ee:d0:b8:81:19:dc:15:4d:
	//						c6:8f:7d:a8:e3:0c:af:15:8a:33:e6:c9:50:9f:4a:
	//						05:b0:14:09:ff:5d:d8:7e:b5
	//					Exponent: 65537 (0x10001)
	//			X509v3 extensions:
	//				X509v3 Subject Key Identifier:
	//					20:31:54:1A:F2:5C:05:FF:D8:65:8B:68:43:79:4F:5E:90:36:F7:B4
	//				X509v3 Authority Key Identifier:
	//					keyid:5F:9D:88:0D:C8:73:E6:54:D4:F8:0D:D8:E6:B0:C1:24:B4:47:C3:55
	//					DirName:/C=GB/O=Certificate Transparency CA/ST=Wales/L=Erw Wen
	//					serial:00
	//
	//				X509v3 Basic Constraints:
	//					CA:FALSE
	//				CT Precertificate SCTs:
	//					Signed Certificate Timestamp:
	//						Version   : v1 (0x0)
	//						Log ID    : DF:1C:2E:C1:15:00:94:52:47:A9:61:68:32:5D:DC:5C:
	//									79:59:E8:F7:C6:D3:88:FC:00:2E:0B:BD:3F:74:D7:64
	//						Timestamp : Apr  5 17:04:16.275 2013 GMT
	//						Extensions: none
	//						Signature : ecdsa-with-SHA256
	//									30:45:02:20:48:2F:67:51:AF:35:DB:A6:54:36:BE:1F:
	//									D6:64:0F:3D:BF:9A:41:42:94:95:92:45:30:28:8F:A3:
	//									E5:E2:3E:06:02:21:00:E4:ED:C0:DB:3A:C5:72:B1:E2:
	//									F5:E8:AB:6A:68:06:53:98:7D:CF:41:02:7D:FE:FF:A1:
	//									05:51:9D:89:ED:BF:08
	//		Signature Algorithm: sha1WithRSAEncryption
	//			 8a:0c:4b:ef:09:9d:47:92:79:af:a0:a2:8e:68:9f:91:e1:c4:
	//			 42:1b:e2:d2:69:a2:ea:6c:a4:e8:21:5d:de:dd:ca:15:04:a1:
	//			 1e:7c:87:c4:b7:7e:80:f0:e9:79:03:52:68:f2:7c:a2:0e:16:
	//			 68:04:ae:55:6f:31:69:81:f9:6a:39:4a:b7:ab:fd:3e:25:5a:
	//			 c0:04:45:13:fe:76:57:0c:67:95:ab:e4:70:31:33:d3:03:f8:
	//			 9f:3a:fa:6b:bc:fc:51:73:19:df:d9:5b:93:42:41:21:1f:63:
	//			 40:35:c3:d0:78:30:7a:68:c6:07:5a:2e:20:c8:9f:36:b8:91:
	//			 0c:a0
	TestEmbeddedCertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIIDWTCCAsKgAwIBAgIBBzANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFIxCzAJBgNVBAYTAkdCMSEwHwYDVQQKExhDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kxDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGfMA0G\n" +
		"CSqGSIb3DQEBAQUAA4GNADCBiQKBgQC+75jnwmh3rjhfdTJaDB0ym+3xj6r015a/\n" +
		"BH634c4VyVui+A7kWL19uG+KSyUhkaeb1wDDjpwDibRc1NyaEgqyHgy0HNDnKAWk\n" +
		"EM2cW9tdSSdyba8XEPYBhzd+olsaHjnu0LiBGdwVTcaPfajjDK8VijPmyVCfSgWw\n" +
		"FAn/Xdh+tQIDAQABo4IBOjCCATYwHQYDVR0OBBYEFCAxVBryXAX/2GWLaEN5T16Q\n" +
		"Nve0MH0GA1UdIwR2MHSAFF+diA3Ic+ZU1PgN2OawwSS0R8NVoVmkVzBVMQswCQYD\n" +
		"VQQGEwJHQjEkMCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4w\n" +
		"DAYDVQQIEwVXYWxlczEQMA4GA1UEBxMHRXJ3IFdlboIBADAJBgNVHRMEAjAAMIGK\n" +
		"BgorBgEEAdZ5AgQCBHwEegB4AHYA3xwuwRUAlFJHqWFoMl3cXHlZ6PfG04j8AC4L\n" +
		"vT9012QAAAE92yffkwAABAMARzBFAiBIL2dRrzXbplQ2vh/WZA89v5pBQpSVkkUw\n" +
		"KI+j5eI+BgIhAOTtwNs6xXKx4vXoq2poBlOYfc9BAn3+/6EFUZ2J7b8IMA0GCSqG\n" +
		"SIb3DQEBBQUAA4GBAIoMS+8JnUeSea+goo5on5HhxEIb4tJpoupspOghXd7dyhUE\n" +
		"oR58h8S3foDw6XkDUmjyfKIOFmgErlVvMWmB+Wo5Srer/T4lWsAERRP+dlcMZ5Wr\n" +
		"5HAxM9MD+J86+mu8/FFzGd/ZW5NCQSEfY0A1w9B4MHpoxgdaLiDInza4kQyg\n" +
		"-----END CERTIFICATE-----\n"

	// TestInvalidEmbeddedCertPEM is a certificate that contains an SCT that is
	// not for the corresponding pre-certificate.  The SCT embedded corresponds
	// to TestInvalidProof.
	//	Certificate:
	//    Data:
	//        Version: 3 (0x2)
	//        Serial Number: 7 (0x7)
	//    Signature Algorithm: sha1WithRSAEncryption
	//        Issuer: C = GB, O = Certificate Transparency CA, ST = Wales, L = Erw Wen
	//        Validity
	//            Not Before: Jun  1 00:00:00 2012 GMT
	//            Not After : Jun  1 00:00:00 2022 GMT
	//        Subject: C = GB, O = Certificate Transparency, ST = Wales, L = Erw Wen
	//        Subject Public Key Info:
	//            Public Key Algorithm: rsaEncryption
	//                Public-Key: (1024 bit)
	//                Modulus:
	//                    00:be:ef:98:e7:c2:68:77:ae:38:5f:75:32:5a:0c:
	//                    1d:32:9b:ed:f1:8f:aa:f4:d7:96:bf:04:7e:b7:e1:
	//                    ce:15:c9:5b:a2:f8:0e:e4:58:bd:7d:b8:6f:8a:4b:
	//                    25:21:91:a7:9b:d7:00:c3:8e:9c:03:89:b4:5c:d4:
	//                    dc:9a:12:0a:b2:1e:0c:b4:1c:d0:e7:28:05:a4:10:
	//                    cd:9c:5b:db:5d:49:27:72:6d:af:17:10:f6:01:87:
	//                    37:7e:a2:5b:1a:1e:39:ee:d0:b8:81:19:dc:15:4d:
	//                    c6:8f:7d:a8:e3:0c:af:15:8a:33:e6:c9:50:9f:4a:
	//                    05:b0:14:09:ff:5d:d8:7e:b5
	//                Exponent: 65537 (0x10001)
	//        X509v3 extensions:
	//            X509v3 Subject Key Identifier:
	//                20:31:54:1A:F2:5C:05:FF:D8:65:8B:68:43:79:4F:5E:90:36:F7:B4
	//            X509v3 Authority Key Identifier:
	//                keyid:5F:9D:88:0D:C8:73:E6:54:D4:F8:0D:D8:E6:B0:C1:24:B4:47:C3:55
	//                DirName:/C=GB/O=Certificate Transparency CA/ST=Wales/L=Erw Wen
	//                serial:00
	//
	//            X509v3 Basic Constraints:
	//                CA:FALSE
	//            CT Precertificate SCTs:
	//                Signed Certificate Timestamp:
	//                    Version   : v1 (0x0)
	//                    Log ID    : DF:1C:2E:C1:15:00:94:52:47:A9:61:68:32:5D:DC:5C:
	//                                79:59:E8:F7:C6:D3:88:FC:00:2E:0B:BD:3F:74:D7:64
	//                    Timestamp : Apr  5 17:04:17.060 2013 GMT
	//                    Extensions: none
	//                    Signature : ecdsa-with-SHA256
	//                                30:45:02:21:00:A6:D3:45:17:F3:39:2D:9E:C5:D2:57:
	//                                AD:F1:C5:97:DC:45:BD:4C:D3:B7:38:56:C6:16:A9:FB:
	//                                99:E5:AE:75:A8:02:20:5E:26:C8:D1:C7:E2:22:FE:8C:
	//                                DA:29:BA:EB:04:A8:34:EE:97:D3:4F:D8:17:18:F1:AA:
	//                                E0:CD:66:F4:B8:A9:3F
	//    Signature Algorithm: sha1WithRSAEncryption
	//         af:28:89:06:38:b0:12:6f:dd:64:5d:d0:62:80:f8:10:6c:ec:
	//         49:4c:f8:22:86:0a:29:d4:f1:7e:6a:a5:7c:5a:58:b2:96:cc:
	//         90:c6:db:f1:22:10:4b:7f:4a:76:d6:fd:df:f2:1a:41:3a:9e:
	//         e7:88:7e:32:a3:c7:a2:07:3c:e6:af:ae:01:b4:1a:a2:3d:ce:
	//         98:f3:ab:5e:c7:5c:e7:59:fa:7c:cc:ab:4f:fa:7a:a7:3e:7d:
	//         98:38:77:c6:d0:f1:de:cd:dd:37:49:00:59:b7:91:90:b2:7f:
	//         85:94:2b:7c:c8:b2:3c:bf:90:30:68:5d:21:43:c4:95:a5:39:
	//         6d:9f
	TestInvalidEmbeddedCertPEM = "-----BEGIN CERTIFICATE-----\n" +
		"MIIDWTCCAsKgAwIBAgIBBzANBgkqhkiG9w0BAQUFADBVMQswCQYDVQQGEwJHQjEk\n" +
		"MCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4wDAYDVQQIEwVX\n" +
		"YWxlczEQMA4GA1UEBxMHRXJ3IFdlbjAeFw0xMjA2MDEwMDAwMDBaFw0yMjA2MDEw\n" +
		"MDAwMDBaMFIxCzAJBgNVBAYTAkdCMSEwHwYDVQQKExhDZXJ0aWZpY2F0ZSBUcmFu\n" +
		"c3BhcmVuY3kxDjAMBgNVBAgTBVdhbGVzMRAwDgYDVQQHEwdFcncgV2VuMIGfMA0G\n" +
		"CSqGSIb3DQEBAQUAA4GNADCBiQKBgQC+75jnwmh3rjhfdTJaDB0ym+3xj6r015a/\n" +
		"BH634c4VyVui+A7kWL19uG+KSyUhkaeb1wDDjpwDibRc1NyaEgqyHgy0HNDnKAWk\n" +
		"EM2cW9tdSSdyba8XEPYBhzd+olsaHjnu0LiBGdwVTcaPfajjDK8VijPmyVCfSgWw\n" +
		"FAn/Xdh+tQIDAQABo4IBOjCCATYwHQYDVR0OBBYEFCAxVBryXAX/2GWLaEN5T16Q\n" +
		"Nve0MH0GA1UdIwR2MHSAFF+diA3Ic+ZU1PgN2OawwSS0R8NVoVmkVzBVMQswCQYD\n" +
		"VQQGEwJHQjEkMCIGA1UEChMbQ2VydGlmaWNhdGUgVHJhbnNwYXJlbmN5IENBMQ4w\n" +
		"DAYDVQQIEwVXYWxlczEQMA4GA1UEBxMHRXJ3IFdlboIBADAJBgNVHRMEAjAAMIGK\n" +
		"BgorBgEEAdZ5AgQCBHwEegB4AHYA3xwuwRUAlFJHqWFoMl3cXHlZ6PfG04j8AC4L\n" +
		"vT9012QAAAE92yfipAAABAMARzBFAiEAptNFF/M5LZ7F0let8cWX3EW9TNO3OFbG\n" +
		"Fqn7meWudagCIF4myNHH4iL+jNopuusEqDTul9NP2BcY8argzWb0uKk/MA0GCSqG\n" +
		"SIb3DQEBBQUAA4GBAK8oiQY4sBJv3WRd0GKA+BBs7ElM+CKGCinU8X5qpXxaWLKW\n" +
		"zJDG2/EiEEt/SnbW/d/yGkE6nueIfjKjx6IHPOavrgG0GqI9zpjzq17HXOdZ+nzM\n" +
		"q0/6eqc+fZg4d8bQ8d7N3TdJAFm3kZCyf4WUK3zIsjy/kDBoXSFDxJWlOW2f\n" +
		"-----END CERTIFICATE-----\n"
)

var (
	// TestCertProof is a TLS-encoded ct.SignedCertificateTimestamp corresponding
	// to TestCertPEM.
	TestCertProof = dh("00df1c2ec11500945247a96168325ddc5c7959e8f7c6d388fc002e0bbd3f74d7" +
		"640000013ddb27ded900000403004730450220606e10ae5c2d5a1b0aed49dc49" +
		"37f48de71a4e9784e9c208dfbfe9ef536cf7f2022100beb29c72d7d06d61d06b" +
		"db38a069469aa86fe12e18bb7cc45689a2c0187ef5a5")

	// TestPreCertProof is a TLS-encoded ct.SignedCertificateTimestamp
	// corresponding to TestPreCertPEM
	TestPreCertProof = dh("00df1c2ec11500945247a96168325ddc5c7959e8f7c6d388fc002e0bbd3f74d7" +
		"640000013ddb27df9300000403004730450220482f6751af35dba65436be1fd6" +
		"640f3dbf9a41429495924530288fa3e5e23e06022100e4edc0db3ac572b1e2f5" +
		"e8ab6a680653987dcf41027dfeffa105519d89edbf08")

	// TestInvalidProof is a TLS-encoded ct.SignedCertificateTimestamp
	// corresponding to the invalid SCT embedded in TestInvalidEmbeddedCertPEM.
	TestInvalidProof = dh("00df1c2ec11500945247a96168325ddc5c7959e8f7c6d388fc002e0bbd3f74d7" +
		"640000013ddb27e2a40000040300473045022100a6d34517f3392d9ec5d257ad" +
		"f1c597dc45bd4cd3b73856c616a9fb99e5ae75a802205e26c8d1c7e222fe8cda" +
		"29baeb04a834ee97d34fd81718f1aae0cd66f4b8a93f")

	// TestCertB64LeafHash is the base64-encoded leaf hash of TestCertPEM with
	// TestCertProof as the corresponding SCT.
	TestCertB64LeafHash = "BKZLZGMbAnDW0gQWjNJNCyS2IgweWn76YW3tFlu3AuY="

	// TestPreCertB64LeafHash is the base64-encoded leaf hash of TestPreCertPEM
	// with TestPreCertProof as the corresponding SCT.
	TestPreCertB64LeafHash = "mbk+d5FDW3oNZJCbOi828rqC3hf+qhigTWVGcXEZq1U="
)

func dh(h string) []byte {
	r, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return r
}
