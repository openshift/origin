// Copyright 2015 Google Inc. All Rights Reserved.
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

package ct

import (
	"bytes"
	"encoding/hex"
	"encoding/pem"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	"github.com/google/certificate-transparency-go/tls"
)

func dh(h string) []byte {
	r, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	return r
}

const (
	defaultSCTLogIDString          string = "iamapublickeyshatwofivesixdigest"
	defaultSCTTimestamp            uint64 = 1234
	defaultSCTSignatureString      string = "\x04\x03\x00\x09signature"
	defaultCertifictateString      string = "certificate"
	defaultPrecertString           string = "precert"
	defaultPrecertIssuerHashString string = "iamapublickeyshatwofivesixdigest"
	defaultPrecertTBSString        string = "tbs"

	defaultCertificateSCTSignatureInputHexString string =
	// version, 1 byte
	"00" +
		// signature type, 1 byte
		"00" +
		// timestamp, 8 bytes
		"00000000000004d2" +
		// entry type, 2 bytes
		"0000" +
		// leaf certificate length, 3 bytes
		"00000b" +
		// leaf certificate, 11 bytes
		"6365727469666963617465" +
		// extensions length, 2 bytes
		"0000" +
		// extensions, 0 bytes
		""

	defaultPrecertSCTSignatureInputHexString string =
	// version, 1 byte
	"00" +
		// signature type, 1 byte
		"00" +
		// timestamp, 8 bytes
		"00000000000004d2" +
		// entry type, 2 bytes
		"0001" +
		// issuer key hash, 32 bytes
		"69616d617075626c69636b657973686174776f66697665736978646967657374" +
		// tbs certificate length, 3 bytes
		"000003" +
		// tbs certificate, 3 bytes
		"746273" +
		// extensions length, 2 bytes
		"0000" +
		// extensions, 0 bytes
		""

	defaultSTHSignedHexString string =
	// version, 1 byte
	"00" +
		// signature type, 1 byte
		"01" +
		// timestamp, 8 bytes
		"0000000000000929" +
		// tree size, 8 bytes
		"0000000000000006" +
		// root hash, 32 bytes
		"696d757374626565786163746c7974686972747974776f62797465736c6f6e67"

	defaultSCTHexString string =
	// version, 1 byte
	"00" +
		// keyid, 32 bytes
		"69616d617075626c69636b657973686174776f66697665736978646967657374" +
		// timestamp, 8 bytes
		"00000000000004d2" +
		// extensions length, 2 bytes
		"0000" +
		// extensions, 0 bytes
		// hash algo, sig algo, 2 bytes
		"0403" +
		// signature length, 2 bytes
		"0009" +
		// signature, 9 bytes
		"7369676e6174757265"

	defaultSCTListHexString string = "0476007400380069616d617075626c69636b657973686174776f6669766573697864696765737400000000000004d20000040300097369676e617475726500380069616d617075626c69636b657973686174776f6669766573697864696765737400000000000004d20000040300097369676e6174757265"
)

func defaultSCTLogID() LogID {
	var id LogID
	copy(id.KeyID[:], defaultSCTLogIDString)
	return id
}

func defaultSCTSignature() DigitallySigned {
	var ds DigitallySigned
	if _, err := tls.Unmarshal([]byte(defaultSCTSignatureString), &ds); err != nil {
		panic(err)
	}
	return ds
}

func defaultSCT() SignedCertificateTimestamp {
	return SignedCertificateTimestamp{
		SCTVersion: V1,
		LogID:      defaultSCTLogID(),
		Timestamp:  defaultSCTTimestamp,
		Extensions: []byte{},
		Signature:  defaultSCTSignature()}
}

func defaultCertificate() []byte {
	return []byte(defaultCertifictateString)
}

func defaultExtensions() []byte {
	return []byte{}
}

func defaultCertificateSCTSignatureInput(t *testing.T) []byte {
	t.Helper()
	r, err := hex.DecodeString(defaultCertificateSCTSignatureInputHexString)
	if err != nil {
		t.Fatalf("failed to decode defaultCertificateSCTSignatureInputHexString: %v", err)
	}
	return r
}

func defaultCertificateLogEntry() LogEntry {
	return LogEntry{
		Index: 1,
		Leaf: MerkleTreeLeaf{
			Version:  V1,
			LeafType: TimestampedEntryLeafType,
			TimestampedEntry: &TimestampedEntry{
				Timestamp: defaultSCTTimestamp,
				EntryType: X509LogEntryType,
				X509Entry: &ASN1Cert{Data: defaultCertificate()},
			},
		},
	}
}

func defaultPrecertSCTSignatureInput(t *testing.T) []byte {
	t.Helper()
	r, err := hex.DecodeString(defaultPrecertSCTSignatureInputHexString)
	if err != nil {
		t.Fatalf("failed to decode defaultPrecertSCTSignatureInputHexString: %v", err)
	}
	return r
}

func defaultPrecertTBS() []byte {
	return []byte(defaultPrecertTBSString)
}

func defaultPrecertIssuerHash() [32]byte {
	var b [32]byte
	copy(b[:], []byte(defaultPrecertIssuerHashString))
	return b
}

func defaultPrecertLogEntry() LogEntry {
	return LogEntry{
		Index: 1,
		Leaf: MerkleTreeLeaf{
			Version:  V1,
			LeafType: TimestampedEntryLeafType,
			TimestampedEntry: &TimestampedEntry{
				Timestamp: defaultSCTTimestamp,
				EntryType: PrecertLogEntryType,
				PrecertEntry: &PreCert{
					IssuerKeyHash:  defaultPrecertIssuerHash(),
					TBSCertificate: defaultPrecertTBS(),
				},
			},
		},
	}
}

func defaultSTH() SignedTreeHead {
	var root SHA256Hash
	copy(root[:], "imustbeexactlythirtytwobyteslong")
	return SignedTreeHead{
		TreeSize:       6,
		Timestamp:      2345,
		SHA256RootHash: root,
		TreeHeadSignature: DigitallySigned{
			Algorithm: tls.SignatureAndHashAlgorithm{
				Hash:      tls.SHA256,
				Signature: tls.ECDSA},
			Signature: []byte("tree_signature"),
		},
	}
}

//////////////////////////////////////////////////////////////////////////////////
// Tests start here:
//////////////////////////////////////////////////////////////////////////////////

func TestSerializeV1SCTSignatureInputForCertificateKAT(t *testing.T) {
	serialized, err := SerializeSCTSignatureInput(defaultSCT(), defaultCertificateLogEntry())
	if err != nil {
		t.Fatalf("Failed to serialize SCT for signing: %v", err)
	}
	if bytes.Compare(serialized, defaultCertificateSCTSignatureInput(t)) != 0 {
		t.Fatalf("Serialized certificate signature input doesn't match expected answer:\n%v\n%v", serialized, defaultCertificateSCTSignatureInput(t))
	}
}

func TestSerializeV1SCTSignatureInputForPrecertKAT(t *testing.T) {
	serialized, err := SerializeSCTSignatureInput(defaultSCT(), defaultPrecertLogEntry())
	if err != nil {
		t.Fatalf("Failed to serialize SCT for signing: %v", err)
	}
	if bytes.Compare(serialized, defaultPrecertSCTSignatureInput(t)) != 0 {
		t.Fatalf("Serialized precertificate signature input doesn't match expected answer:\n%v\n%v", serialized, defaultPrecertSCTSignatureInput(t))
	}
}

func TestSerializeV1SCTJSONSignature(t *testing.T) {
	entry := LogEntry{Leaf: *CreateJSONMerkleTreeLeaf("data", defaultSCT().Timestamp)}
	expected := dh(
		// version, 1 byte
		"00" +
			// signature type, 1 byte
			"00" +
			// timestamp, 8 bytes
			"00000000000004d2" +
			// entry type, 2 bytes
			"8000" +
			// tbs certificate length, 18 bytes
			"000012" +
			// { "data": "data" }, 3 bytes
			"7b202264617461223a20226461746122207d" +
			// extensions length, 2 bytes
			"0000" +
			// extensions, 0 bytes
			"")
	serialized, err := SerializeSCTSignatureInput(defaultSCT(), entry)
	if err != nil {
		t.Fatalf("Failed to serialize SCT for signing: %v", err)
	}
	if !bytes.Equal(serialized, expected) {
		t.Fatalf("Serialized JSON signature :\n%x, want\n%x", serialized, expected)
	}
}

func TestSerializeV1STHSignatureKAT(t *testing.T) {
	b, err := SerializeSTHSignatureInput(defaultSTH())
	if err != nil {
		t.Fatalf("Failed to serialize defaultSTH: %v", err)
	}
	if bytes.Compare(b, mustDehex(t, defaultSTHSignedHexString)) != 0 {
		t.Fatalf("defaultSTH incorrectly serialized, expected:\n%v\ngot:\n%v", mustDehex(t, defaultSTHSignedHexString), b)
	}
}

func TestMarshalDigitallySigned(t *testing.T) {
	b, err := tls.Marshal(
		DigitallySigned{
			Algorithm: tls.SignatureAndHashAlgorithm{
				Hash:      tls.SHA512,
				Signature: tls.ECDSA},
			Signature: []byte("signature")})
	if err != nil {
		t.Fatalf("Failed to marshal DigitallySigned struct: %v", err)
	}
	if b[0] != byte(tls.SHA512) {
		t.Fatalf("Expected b[0] == SHA512, but found %v", tls.HashAlgorithm(b[0]))
	}
	if b[1] != byte(tls.ECDSA) {
		t.Fatalf("Expected b[1] == ECDSA, but found %v", tls.SignatureAlgorithm(b[1]))
	}
	if b[2] != 0x00 || b[3] != 0x09 {
		t.Fatalf("Found incorrect length bytes, expected (0x00, 0x09) found %v", b[2:3])
	}
	if string(b[4:]) != "signature" {
		t.Fatalf("Found incorrect signature bytes, expected %v, found %v", []byte("signature"), b[4:])
	}
}

func TestUnmarshalDigitallySigned(t *testing.T) {
	var ds DigitallySigned
	if _, err := tls.Unmarshal([]byte("\x01\x02\x00\x0aSiGnAtUrE!"), &ds); err != nil {
		t.Fatalf("Failed to unmarshal DigitallySigned: %v", err)
	}
	if ds.Algorithm.Hash != tls.MD5 {
		t.Fatalf("Expected HashAlgorithm %v, but got %v", tls.MD5, ds.Algorithm.Hash)
	}
	if ds.Algorithm.Signature != tls.DSA {
		t.Fatalf("Expected SignatureAlgorithm %v, but got %v", tls.DSA, ds.Algorithm.Signature)
	}
	if string(ds.Signature) != "SiGnAtUrE!" {
		t.Fatalf("Expected Signature %v, but got %v", []byte("SiGnAtUrE!"), ds.Signature)
	}
}

func TestMarshalUnmarshalSCTRoundTrip(t *testing.T) {
	sctIn := defaultSCT()
	b, err := tls.Marshal(sctIn)
	if err != nil {
		t.Fatalf("tls.Marshal(SCT)=nil,%v; want no error", err)
	}
	var sctOut SignedCertificateTimestamp
	if _, err := tls.Unmarshal(b, &sctOut); err != nil {
		t.Errorf("tls.Unmarshal(%s)=nil,%v; want %+v,nil", hex.EncodeToString(b), err, sctIn)
	} else if !reflect.DeepEqual(sctIn, sctOut) {
		t.Errorf("tls.Unmarshal(%s)=%v,nil; want %+v,nil", hex.EncodeToString(b), sctOut, sctIn)
	}
}

func TestMarshalSCT(t *testing.T) {
	b, err := tls.Marshal(defaultSCT())
	if err != nil {
		t.Errorf("tls.Marshal(defaultSCT)=nil,%v; want %s", err, defaultSCTHexString)
	} else if !bytes.Equal(dh(defaultSCTHexString), b) {
		t.Errorf("tls.Marshal(defaultSCT)=%s,nil; want %s", hex.EncodeToString(b), defaultSCTHexString)
	}
}

func TestUnmarshalSCT(t *testing.T) {
	want := defaultSCT()
	var got SignedCertificateTimestamp
	if _, err := tls.Unmarshal(dh(defaultSCTHexString), &got); err != nil {
		t.Errorf("tls.Unmarshal(%s)=nil,%v; want %+v,nil", defaultSCTHexString, err, want)
	} else if !reflect.DeepEqual(got, want) {
		t.Errorf("tls.Unmarshal(%s)=%+v,nil; want %+v,nil", defaultSCTHexString, got, want)
	}
}

func TestX509MerkleTreeLeafHash(t *testing.T) {
	certFile := "./testdata/test-cert.pem"
	sctFile := "./testdata/test-cert.proof"
	certB, err := ioutil.ReadFile(certFile)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", certFile, err)
	}
	certDER, _ := pem.Decode(certB)

	sctB, err := ioutil.ReadFile(sctFile)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", sctFile, err)
	}
	var sct SignedCertificateTimestamp
	if _, err := tls.Unmarshal(sctB, &sct); err != nil {
		t.Fatalf("Failed to deserialize SCT: %v", err)
	}

	leaf := CreateX509MerkleTreeLeaf(ASN1Cert{Data: certDER.Bytes}, sct.Timestamp)
	b, err := tls.Marshal(*leaf)
	if err != nil {
		t.Fatalf("Failed to Serialize x509 leaf: %v", err)
	}

	leafBytes := dh("00000000013ddb27ded900000002ce308202ca30820233a003020102020106300d06092a864886f70d01010505003055310b300906035504061302474231243022060355040a131b4365727469666963617465205472616e73706172656e6379204341310e300c0603550408130557616c65733110300e060355040713074572772057656e301e170d3132303630313030303030305a170d3232303630313030303030305a3052310b30090603550406130247423121301f060355040a13184365727469666963617465205472616e73706172656e6379310e300c0603550408130557616c65733110300e060355040713074572772057656e30819f300d06092a864886f70d010101050003818d0030818902818100b1fa37936111f8792da2081c3fe41925008531dc7f2c657bd9e1de4704160b4c9f19d54ada4470404c1c51341b8f1f7538dddd28d9aca48369fc5646ddcc7617f8168aae5b41d43331fca2dadfc804d57208949061f9eef902ca47ce88c644e000f06eeeccabdc9dd2f68a22ccb09dc76e0dbc73527765b1a37a8c676253dcc10203010001a381ac3081a9301d0603551d0e041604146a0d982a3b62c44b6d2ef4e9bb7a01aa9cb798e2307d0603551d230476307480145f9d880dc873e654d4f80dd8e6b0c124b447c355a159a4573055310b300906035504061302474231243022060355040a131b4365727469666963617465205472616e73706172656e6379204341310e300c0603550408130557616c65733110300e060355040713074572772057656e82010030090603551d1304023000300d06092a864886f70d010105050003818100171cd84aac414a9a030f22aac8f688b081b2709b848b4e5511406cd707fed028597a9faefc2eee2978d633aaac14ed3235197da87e0f71b8875f1ac9e78b281749ddedd007e3ecf50645f8cbf667256cd6a1647b5e13203bb8582de7d6696f656d1c60b95f456b7fcf338571908f1c69727d24c4fccd249295795814d1dac0e60000")
	if !bytes.Equal(b, leafBytes) {
		t.Errorf("CreateX509MerkleTreeLeaf(): got\n %x, want\n%x", b, sctB)
	}

}

func TestJSONMerkleTreeLeaf(t *testing.T) {
	data := `CioaINV25GV8X4a6M6Q10avSLP9PYd5N8MwWxQvWU7E2CzZ8IgYI0KnavAUSWAoIZDc1NjMzMzMSTAgEEAMaRjBEAiBQlnp6Q3di86g8M3l5gz+9qls/Cz1+KJ+tK/jpaBtUCgIgXaJ94uLsnChA1NY7ocGwKrQwPU688hwaZ5L/DboV4mQ=2`
	timestamp := uint64(1469664866615)
	leaf := CreateJSONMerkleTreeLeaf(data, timestamp)
	b, err := tls.Marshal(*leaf)
	if err != nil {
		t.Fatalf("Failed to Serialize x509 leaf: %v", err)
	}
	leafBytes := dh("0000000001562eda313780000000c67b202264617461223a202243696f61494e563235475638583461364d365131306176534c5039505964354e384d77577851765755374532437a5a3849675949304b6e617641555357416f495a4463314e6a4d7a4d7a4d535441674545414d61526a4245416942516c6e703651336469383667384d336c35677a2b39716c735c2f437a312b4b4a2b744b5c2f6a70614274554367496758614a3934754c736e436841314e59376f6347774b72517750553638386877615a354c5c2f44626f56346d513d3222207d0000")

	if !bytes.Equal(b, leafBytes) {
		t.Errorf("CreateJSONMerkleTreeLeaf(): got\n%x, want\n%x", b, leafBytes)
	}
}

func TestLogEntryFromLeaf(t *testing.T) {
	const (
		// Cert example taken from entry #1 in argon2018 log
		leafDER = "308204ef308202d7a00302010202070556658a503cca300d06092a864886f70d01010b0500307f310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793123302106035504030c1a4d657267652044656c617920496e7465726d6564696174652031301e170d3137303831303132343331355a170d3138303333313038333231375a3063310b3009060355040613024742310f300d06035504070c064c6f6e646f6e31283026060355040a0c1f476f6f676c65204365727469666963617465205472616e73706172656e637931193017060355040513103135303233363839393537353331363230820122300d06092a864886f70d01010105000382010f003082010a0282010100a2fb53365dfbcefea77e1d65bc40f34f7919fcae85d82d3003428199f0c893fca95ba139156fd5e9a3bd84dc6dab8e74151fde6dd25b31526c85719bbf8990f3d6b21bb7f321306f6ddc50b96e8917fa103b388a00e1e954ee0232a9f9fb2efa8c9f9196a7fe84dad1f66b5d36127c71c9dcf25a04acd7bfda7866dfb77498c63a7ae9e7d0772fe9ba938a9ff6c0209196988158e6ea055fe967dd7599ef4bd7f306ded231cca10d89b4d6de40916e615d1d4cc6032585822a650743e34735d464fc0d544d1fad8c293df22f4a55ce3fbfb55d90cdc5ab84695a5a13d46f3176f143d9d28f60dca841eac603d30cec830a62feec091c927e6c781df330f14ca10203010001a3818b30818830130603551d25040c300a06082b0601050507030130230603551d11041c301a8218666c6f776572732d746f2d7468652d776f726c642e636f6d300c0603551d130101ff04023000301f0603551d23041830168014e93c04e1802fc284132d26709ef2fd1acfaafec6301d0603551d0e041604142f3948061fe546939f5e8dbc3fe4c0a1fbaab6b7300d06092a864886f70d01010b0500038202010052a2480c754c51cfdc9f99a82a8eb7c34e5e2bdcdfdd7543fadadb578083416d34ebb87fea3c90baf97f06be5baf5c41101ad1bdd2a2f554de6a3e8cd5ff3d78354badad01032a007d2eaf03590ee5397e223b2936f0c8b59c0407079c8975ffb34eb1cfe784cf3bc45e198a601473537f1ef382e0b5311d2ddf430ade7cfd28900ec9d91c1db49a6b2fb1b9e13b94135fed978d646e048b2fa9dc36ef5821cea8ebbed38d4c2d7811e9660f23d7636b295caccdc945a010a4c364fd7e7480aa5282d28fc46ce7f4f636ef2cc57c8bb1aee5da79bc6107205d4abcd3fb09a1db023ba4e8e9f34ae36ff5b2672fc2a14af8d23d67a437b3eb507ca90f73121841ab1498ab712d18063244dc3514bfffbaf6d45acdfc5316a248589a04b79b2abbca454e2e21f9b487e21eea21565c99ebc1013b87253c91f43ac6d2d2dea7090877c2a7404bce2545662ce005dc12eb57b1efe7145af8070b5dfa86736664a644a9c0f7e7c38d715cf874b818d519927eddc69b55c781b6e0a6eef8f3e46b9e059105b7932a978704e924904dbaa9583f3dd606467f4cc41589b702f1a02d517d3cd93b55d67d0b379e2527fded951be9dfb86d473613e6d9b8399ef5174d3e03185bd3cb4ea859b465c6c49010d4119d613a60878c0e453f17cfa3ce925e10f6e0a5adb745cebe218c3c82627a120e2907eeb9ec5307664474093cbc92d65fc7"
		leafCA  = "308205c8308203b0a00302010202021001300d06092a864886f70d0101050500307d310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793121301f06035504030c184d657267652044656c6179204d6f6e69746f7220526f6f74301e170d3134303731373132323633305a170d3139303731363132323633305a307f310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793123302106035504030c1a4d657267652044656c617920496e7465726d656469617465203130820222300d06092a864886f70d01010105000382020f003082020a0282020100c1e874feff9aeef303bbfa63453881faaf8dc1c22c09641daf430381f33bc157bf6c4c8a8d57b1abc792859d20f2191509c597c437b14673dea5af4bea14396dd436dc620555d7953e0ede01f7ffb44f3ff7cde64ed245634e0df0aafce9c3ac5eb63d8de1d969cac8854195403a9f9d1d4c3dceedf1351edd945743bc54ab745b204fb5259fe3edf695c2cf90b886c48ff680b744fec2691b4345242b31c36f3118b727c5de5c25ec1aa30a4c2461c5119ef6bb90d816e6e44c5b9955bfa2ed3416bf6e4a53c92fafac0d1f0b8bf3d35be0d4f61ca05d28fc662dfa588fba3ee0380470c012ded9e51bbf1e7a25efa745784c49d05eaf8dcee0527361ec913126005e972cf4b863914f8361582ed24563ff9e03c52e8a9ca3264c56c186b4ec52b7e695ce42ae17ec7ae0257131e1dbf48f2dde242e6e91ea304988135a15482b05fc091355328b39e586e8dd3a4a3a14cb97eef68f9f69728c291f2195d2cce73d4ae90845b1bfc5fae040b94fc359a29511981b9966aeb56d3a7c5e48f8eca815e5be86b3d36e6a27e0e2c4dee6e30f12a7c936b8c98cad5928aca238dfc39cf9f2c5246cbbbb280cb6f99eb49bfd1d78089539072c164c7083371746dedbc4dec1cb9439073af3f2e60f8c505f067961a8c539454fc5341158eccc78532f3e39c3187c9439fc0ff88ee957131d478df063dd50b2ad3fe7a070e905e3868b0203010001a350304e301d0603551d0e04160414e93c04e1802fc284132d26709ef2fd1acfaafec6301f0603551d23041830168014f35f7b7549e37841396a20b67c6b4c5cc93d5841300c0603551d13040530030101ff300d06092a864886f70d010105050003820201000858cbd545f2e92e09906ac39f3d55c13607a651436386c6e90f128773a0eb3f4725d8af35fb7f880436b481f2cf47801825de54f13f8f5920bf3d916e753141de5e59d2debbfc3fc226721f15a16d3b7a4618ea0551639f1d2cb7b9faad1b7e070f23a6e8197c3d7549bba6553fd5db419ce399477f6a0481b90f51c9d307d82cb05cf967828a1ace65207cf86b6d16792245dcf24b4c179f91184736e7e2fcb863a4b5c89b0ac2f368390a10594b95c856e259c77564316898cf87a6817d18585fc976d681d9d510ef2ad37e8ad0e49f5bd499c9ec7fe8f43b17dffb9b7d0dfd8300c1c5389c9ea0be4370dcbf78bd3efc2308d250b866bbca031c0c49ff77a7a5420daa1f1b6a444d366653974c2d179c3009871ee6c89140fca9efdf23bd4b88c6ebaeb9286f58f3cfc21e4874f182d1ecd6058919b03b18db7b795be9cb25fc5166a945ef8e1133cd60312a3234f4649df6166407cbb5ecc838e9e118c05dee7c896a9987655ae7e349cd8166e68d34dea3b4ae892a9f2385053271e860b542be3650503974f3bb6f2688375ab28487da6751d0f2c3a35e78efe30b19d57808ebea2c4453990ad81eb96289c0f99c5080f82092bc6123a340c63a617f3bc4adc1298d88a278a693ef93688611d3b4eded0b6d023ed9f6c2ea8836483197525b1b1bce70a90c3403b094d5f412aa1141b9965ab8314c52f772deffc1008c"
		rootCA  = "308205cd308203b5a0030201020209009ed3ccb1d12ca272300d06092a864886f70d0101050500307d310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793121301f06035504030c184d657267652044656c6179204d6f6e69746f7220526f6f74301e170d3134303731373132303534335a170d3431313230323132303534335a307d310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793121301f06035504030c184d657267652044656c6179204d6f6e69746f7220526f6f7430820222300d06092a864886f70d01010105000382020f003082020a0282020100aa161cf2205ed81ac565483cda426a3db2e588fdb758b17b93ea8d68495d534a01ba4f6cd1c0fc0a128af79c066dc54c3f437e05ba275ee61dbf9cbdb2928183738139397b6189ae738fef2b9b609a6dd8e0b0d0e20b243db936c029cdc2220af2c0e1a5e4aa41a006af458957e2b1178d27156ef0cb717e16d54025d97f43e9916fb240fb85f7d579462fa0ac76c76256843750bf1ccdfeb76c8c47886477644d5ec3235628adf6a09c8488bfa5036de717908151a6b585f273dd9fb5332b9af76e8fbfa91eaf4311816dde27c5c44f2fd06cc2204d7147f77ba6b16a2a5fca470023614729538bee6b3cb07264713832aec161550eb501906802215223acc2564ad1f98bb5934924eb56d383fc7598be45c89d995281c0efb0d206d29a6d25a10a48fe235332379c5ca69e83599faa677dd20823f5c84a961255eca5d4871d54ca1df0774aa117b0f42cd6e9fda7e8a48a53923c5f94043353544e644b5a6562e5cef9fc2bd2fcfcce3323335cf7fe7c4d83c1b7f839c4790192d3ba9aa9f32093aa8ee7cbe708059d538dc663cca1b825331aa836754a0d13de63bf65b6e2044dcdf041f1a0c5a9c3c38fe74cf576d451c23eaa519db32ef9e039bd848a194c3b5e41a55642dc283ddbd73d1dd97ae6951de18ad89d005007fae7e88bc7a3cce8b7ccc49603a0db67c76d58a28d4b77aa7460801e34377d0c5e4606c2e25b0203010001a350304e301d0603551d0e04160414f35f7b7549e37841396a20b67c6b4c5cc93d5841301f0603551d23041830168014f35f7b7549e37841396a20b67c6b4c5cc93d5841300c0603551d13040530030101ff300d06092a864886f70d01010505000382020100771cfea34579a97520d8c2423d68ecd07891f8c7f1c38bf3cd30ea9d3637bfc5d373532ec7656558bef4950646750c6fe085c52d9ffc09e66ebfa2b067de7727cb381d2b25db58b9c2197fd5eb53e020f429b86a3b1f37a07a761a66a5b3ecd797c46695a37ff2d47c54126be6bd28a2a103357227c6b73f7f689b09b48927e6e9a52267a728a115d4bcbb477533dc28f3fc57da735a3ec54fbc36990b17febb7e46b324208c1fa7425a0cba48bdc0381ea8285215261c3c483f2fa6d1da0dba4949107189f32d728a7ff395d43430af3b8ce4be5075bcf67d66661941dc8be37340f8f9282b2d2aadd69065932ad49769f8bc7fc9e5f6e79ff392420ba78de3172778e2b67e4df18440dd561e5a7844c8efa06c0e5f5b8695fa069114a00518fb4c19f9d7855831b5eeebced14b8598daffa49f2dcf505bff6417d84b28e83599d4e0371ef64b2d82ffa068a31044f7322fee2f654ec357c9c121f3458a509728c37f5673412ad0d5e76aa6b4eb1582181a8be404d3dc35e71edd83ee388087d6147c4d86f1cacacface0104df1f4b100c2ceb1be4d1851c4f31e7c4409262185878f23cceb30790143f0d5bd80d2c0ed4260aaa312597d950aaf3c8bcfc812d9a56e8d160dd772a494743710a7e478a046d8a5d505ee6b8cc37fec09dfd4cb57c6c4d8e8ef2a22e1d9e50957852633138d722253e51ba77b0069d38812207a"
		noExts  = "0000"
		// Precert example taken from entry #2 in argon2018 log
		issuerKeyHash = "e37689003073a0c649cc656de946c03174d25c566fe3c3805b846f5236943798"
		precertTBS    = "0002db308202d7a0030201020207055667377e8bcc300d06092a864886f70d01010b0500307f310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793123302106035504030c1a4d657267652044656c617920496e7465726d6564696174652031301e170d3137303831303134343331365a170d3138303432363134323430315a3063310b3009060355040613024742310f300d06035504070c064c6f6e646f6e31283026060355040a0c1f476f6f676c65204365727469666963617465205472616e73706172656e637931193017060355040513103135303233373631393632313337303830820122300d06092a864886f70d01010105000382010f003082010a0282010100dea100ff02f31ae6f76f9c26525afcdd0ef6eef780d72b4b1c0ff14fc7ac021852d6f34af20713d05fea2c2e1a4b488b3849c2511cf30fcd2e61d9e7392557498a4ff600c8fdc912f05c8a583a5f2b6a3d3320c6cc10b7eed502de392d11b3c4d57fa6e3ddc69d3f73305bb6441a0359bd526272784523ae5319cffd2993abba54d26c4c1b760c8660b65161a349e415207a6fbb20d02ce13054e6ffa7776bccfd26c3e4220e0e504f102f352260aaa9864411f7eeae8f6071c9ba54b83d11af43ab58dcb6a7d9053654b98b165fb84a27c78361d957c70a064f45bf4501ef744302e497839a34222e94b3018e587c70c130208976d9f80cf6741a95abfa6b810203010001a3818b30818830130603551d25040c300a06082b0601050507030130230603551d11041c301a8218666c6f776572732d746f2d7468652d776f726c642e636f6d300c0603551d130101ff04023000301f0603551d23041830168014e93c04e1802fc284132d26709ef2fd1acfaafec6301d0603551d0e04160414df25c220250d548e08341c26cadc5effc177841c"
		precertDER    = "30820504308202eca0030201020207055667377e8bcc300d06092a864886f70d01010b0500307f310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793123302106035504030c1a4d657267652044656c617920496e7465726d6564696174652031301e170d3137303831303134343331365a170d3138303432363134323430315a3063310b3009060355040613024742310f300d06035504070c064c6f6e646f6e31283026060355040a0c1f476f6f676c65204365727469666963617465205472616e73706172656e637931193017060355040513103135303233373631393632313337303830820122300d06092a864886f70d01010105000382010f003082010a0282010100dea100ff02f31ae6f76f9c26525afcdd0ef6eef780d72b4b1c0ff14fc7ac021852d6f34af20713d05fea2c2e1a4b488b3849c2511cf30fcd2e61d9e7392557498a4ff600c8fdc912f05c8a583a5f2b6a3d3320c6cc10b7eed502de392d11b3c4d57fa6e3ddc69d3f73305bb6441a0359bd526272784523ae5319cffd2993abba54d26c4c1b760c8660b65161a349e415207a6fbb20d02ce13054e6ffa7776bccfd26c3e4220e0e504f102f352260aaa9864411f7eeae8f6071c9ba54b83d11af43ab58dcb6a7d9053654b98b165fb84a27c78361d957c70a064f45bf4501ef744302e497839a34222e94b3018e587c70c130208976d9f80cf6741a95abfa6b810203010001a381a030819d30130603551d25040c300a06082b0601050507030130230603551d11041c301a8218666c6f776572732d746f2d7468652d776f726c642e636f6d300c0603551d130101ff04023000301f0603551d23041830168014e93c04e1802fc284132d26709ef2fd1acfaafec6301d0603551d0e04160414df25c220250d548e08341c26cadc5effc177841c3013060a2b06010401d6790204030101ff04020500300d06092a864886f70d01010b05000382020100ae9ca16ec19bb469d08628b1296f50e3e15b362e2b18c691b11eef3af9ce655bf74e0c21c84f6091132851ba78465c3ae97a1409ee7505395d4e7e0318189029a12bf1c3ba2b6f3231c7aac13dbbfaade8d56f0fbe91d32440ad0ab816184c72392154275ead8418cc62e4e2b08de1b14acb6b27c0f36fa586feb875666f46d232a32ef022440d52cdd8bd31a42de55bfa77de8742816f086830b07eedbde545af5a2b9dd17bd49ded508589a0673f6e0d55f210818422093fd10939f0c81521ca654958e6e01b76ef8c7380bdb331e67d44ccb18a83ed04d97d463c37c7cbc592768e2373e198a1d64be3bd22d1833994706797461d05a85e779cd6e2b4b2b14e81d1eca454f29780c47a7366041ace1a48319eff3f1f04bbd471d5125774ef050e47bf664a98101b7be3337bb786b760a92be46488c6a15f72972a4b7c932c736311f0ac1d40920580329657f00e26cfb6d3b1db1eb7a95952fbcbfcbaf9d17587f03aeb9c3b403d1dccad895316658d35fe385fc5a62b60db36e3f07c4798314936aeb3f40094aee9ec1350ea8f68d1aeb41b211ecd0c9e29c5fa2d6576bcb2ad5ec8cc936e1f5a127afa0de3ae490b914adaa733f18ed9348d497e10ba4aa3008f84deec6976292dc0d3c2aa523602188916dd468b47f3d571e71fe51cd293c805d1280a53ab9f519a616d889303be461354edfc29dc3cc85d8570264cf4"
		precertCA     = "308205c8308203b0a00302010202021001300d06092a864886f70d0101050500307d310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793121301f06035504030c184d657267652044656c6179204d6f6e69746f7220526f6f74301e170d3134303731373132323633305a170d3139303731363132323633305a307f310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793123302106035504030c1a4d657267652044656c617920496e7465726d656469617465203130820222300d06092a864886f70d01010105000382020f003082020a0282020100c1e874feff9aeef303bbfa63453881faaf8dc1c22c09641daf430381f33bc157bf6c4c8a8d57b1abc792859d20f2191509c597c437b14673dea5af4bea14396dd436dc620555d7953e0ede01f7ffb44f3ff7cde64ed245634e0df0aafce9c3ac5eb63d8de1d969cac8854195403a9f9d1d4c3dceedf1351edd945743bc54ab745b204fb5259fe3edf695c2cf90b886c48ff680b744fec2691b4345242b31c36f3118b727c5de5c25ec1aa30a4c2461c5119ef6bb90d816e6e44c5b9955bfa2ed3416bf6e4a53c92fafac0d1f0b8bf3d35be0d4f61ca05d28fc662dfa588fba3ee0380470c012ded9e51bbf1e7a25efa745784c49d05eaf8dcee0527361ec913126005e972cf4b863914f8361582ed24563ff9e03c52e8a9ca3264c56c186b4ec52b7e695ce42ae17ec7ae0257131e1dbf48f2dde242e6e91ea304988135a15482b05fc091355328b39e586e8dd3a4a3a14cb97eef68f9f69728c291f2195d2cce73d4ae90845b1bfc5fae040b94fc359a29511981b9966aeb56d3a7c5e48f8eca815e5be86b3d36e6a27e0e2c4dee6e30f12a7c936b8c98cad5928aca238dfc39cf9f2c5246cbbbb280cb6f99eb49bfd1d78089539072c164c7083371746dedbc4dec1cb9439073af3f2e60f8c505f067961a8c539454fc5341158eccc78532f3e39c3187c9439fc0ff88ee957131d478df063dd50b2ad3fe7a070e905e3868b0203010001a350304e301d0603551d0e04160414e93c04e1802fc284132d26709ef2fd1acfaafec6301f0603551d23041830168014f35f7b7549e37841396a20b67c6b4c5cc93d5841300c0603551d13040530030101ff300d06092a864886f70d010105050003820201000858cbd545f2e92e09906ac39f3d55c13607a651436386c6e90f128773a0eb3f4725d8af35fb7f880436b481f2cf47801825de54f13f8f5920bf3d916e753141de5e59d2debbfc3fc226721f15a16d3b7a4618ea0551639f1d2cb7b9faad1b7e070f23a6e8197c3d7549bba6553fd5db419ce399477f6a0481b90f51c9d307d82cb05cf967828a1ace65207cf86b6d16792245dcf24b4c179f91184736e7e2fcb863a4b5c89b0ac2f368390a10594b95c856e259c77564316898cf87a6817d18585fc976d681d9d510ef2ad37e8ad0e49f5bd499c9ec7fe8f43b17dffb9b7d0dfd8300c1c5389c9ea0be4370dcbf78bd3efc2308d250b866bbca031c0c49ff77a7a5420daa1f1b6a444d366653974c2d179c3009871ee6c89140fca9efdf23bd4b88c6ebaeb9286f58f3cfc21e4874f182d1ecd6058919b03b18db7b795be9cb25fc5166a945ef8e1133cd60312a3234f4649df6166407cbb5ecc838e9e118c05dee7c896a9987655ae7e349cd8166e68d34dea3b4ae892a9f2385053271e860b542be3650503974f3bb6f2688375ab28487da6751d0f2c3a35e78efe30b19d57808ebea2c4453990ad81eb96289c0f99c5080f82092bc6123a340c63a617f3bc4adc1298d88a278a693ef93688611d3b4eded0b6d023ed9f6c2ea8836483197525b1b1bce70a90c3403b094d5f412aa1141b9965ab8314c52f772deffc1008c"
		precertRoot   = "308205cd308203b5a0030201020209009ed3ccb1d12ca272300d06092a864886f70d0101050500307d310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793121301f06035504030c184d657267652044656c6179204d6f6e69746f7220526f6f74301e170d3134303731373132303534335a170d3431313230323132303534335a307d310b3009060355040613024742310f300d06035504080c064c6f6e646f6e31173015060355040a0c0e476f6f676c6520554b204c74642e3121301f060355040b0c184365727469666963617465205472616e73706172656e63793121301f06035504030c184d657267652044656c6179204d6f6e69746f7220526f6f7430820222300d06092a864886f70d01010105000382020f003082020a0282020100aa161cf2205ed81ac565483cda426a3db2e588fdb758b17b93ea8d68495d534a01ba4f6cd1c0fc0a128af79c066dc54c3f437e05ba275ee61dbf9cbdb2928183738139397b6189ae738fef2b9b609a6dd8e0b0d0e20b243db936c029cdc2220af2c0e1a5e4aa41a006af458957e2b1178d27156ef0cb717e16d54025d97f43e9916fb240fb85f7d579462fa0ac76c76256843750bf1ccdfeb76c8c47886477644d5ec3235628adf6a09c8488bfa5036de717908151a6b585f273dd9fb5332b9af76e8fbfa91eaf4311816dde27c5c44f2fd06cc2204d7147f77ba6b16a2a5fca470023614729538bee6b3cb07264713832aec161550eb501906802215223acc2564ad1f98bb5934924eb56d383fc7598be45c89d995281c0efb0d206d29a6d25a10a48fe235332379c5ca69e83599faa677dd20823f5c84a961255eca5d4871d54ca1df0774aa117b0f42cd6e9fda7e8a48a53923c5f94043353544e644b5a6562e5cef9fc2bd2fcfcce3323335cf7fe7c4d83c1b7f839c4790192d3ba9aa9f32093aa8ee7cbe708059d538dc663cca1b825331aa836754a0d13de63bf65b6e2044dcdf041f1a0c5a9c3c38fe74cf576d451c23eaa519db32ef9e039bd848a194c3b5e41a55642dc283ddbd73d1dd97ae6951de18ad89d005007fae7e88bc7a3cce8b7ccc49603a0db67c76d58a28d4b77aa7460801e34377d0c5e4606c2e25b0203010001a350304e301d0603551d0e04160414f35f7b7549e37841396a20b67c6b4c5cc93d5841301f0603551d23041830168014f35f7b7549e37841396a20b67c6b4c5cc93d5841300c0603551d13040530030101ff300d06092a864886f70d01010505000382020100771cfea34579a97520d8c2423d68ecd07891f8c7f1c38bf3cd30ea9d3637bfc5d373532ec7656558bef4950646750c6fe085c52d9ffc09e66ebfa2b067de7727cb381d2b25db58b9c2197fd5eb53e020f429b86a3b1f37a07a761a66a5b3ecd797c46695a37ff2d47c54126be6bd28a2a103357227c6b73f7f689b09b48927e6e9a52267a728a115d4bcbb477533dc28f3fc57da735a3ec54fbc36990b17febb7e46b324208c1fa7425a0cba48bdc0381ea8285215261c3c483f2fa6d1da0dba4949107189f32d728a7ff395d43430af3b8ce4be5075bcf67d66661941dc8be37340f8f9282b2d2aadd69065932ad49769f8bc7fc9e5f6e79ff392420ba78de3172778e2b67e4df18440dd561e5a7844c8efa06c0e5f5b8695fa069114a00518fb4c19f9d7855831b5eeebced14b8598daffa49f2dcf505bff6417d84b28e83599d4e0371ef64b2d82ffa068a31044f7322fee2f654ec357c9c121f3458a509728c37f5673412ad0d5e76aa6b4eb1582181a8be404d3dc35e71edd83ee388087d6147c4d86f1cacacface0104df1f4b100c2ceb1be4d1851c4f31e7c4409262185878f23cceb30790143f0d5bd80d2c0ed4260aaa312597d950aaf3c8bcfc812d9a56e8d160dd772a494743710a7e478a046d8a5d505ee6b8cc37fec09dfd4cb57c6c4d8e8ef2a22e1d9e50957852633138d722253e51ba77b0069d38812207a"
	)

	corruptedLeafDER := "aaaaaaaaaa" + leafDER[10:]
	corruptedPrecertTBS := precertTBS[:6] + "aaaaaaaaaa" + precertTBS[16:]

	var tests = []struct {
		leaf        LeafEntry
		wantCert    bool
		wantPrecert bool
		wantErr     string
	}{
		{
			leaf:    LeafEntry{},
			wantErr: "failed to unmarshal MerkleTreeLeaf",
		},
		{
			leaf: LeafEntry{
				// {version + leaf_type + timestamp + entry_type + len + cert + exts}
				LeafInput: dh("00" + "00" + "0000015dcc2b99c8" + "0000" + "0004f3" + leafDER + noExts),
				ExtraData: dh("000ba3" + "0005cc" + leafCA + "0005d1" + rootCA),
			},
			wantCert: true,
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc2b99c8" + "0000" + "0004f3" + corruptedLeafDER + noExts),
				ExtraData: dh("000ba3" + "0005cc" + leafCA + "0005d1" + rootCA),
			},
			wantErr: "failed to parse certificate",
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc2b99c8" + "0000" + "0004f3" + leafDER + noExts + "ff"),
				ExtraData: dh("000ba3" + "0005cc" + leafCA + "0005d1" + rootCA),
			},
			wantErr: "MerkleTreeLeaf: trailing data",
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc2b99c8" + "0000" + "0004f3" + leafDER + noExts),
				ExtraData: dh("000ba3" + "0005cc" + leafCA + "0005d1" + rootCA + "00"),
			},
			wantErr: "CertificateChain: trailing data",
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc2b99c8" + "0000" + "0004f3" + leafDER + noExts),
			},
			wantErr: "failed to unmarshal CertificateChain",
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc2b99c8" + "8000" + "0004f3" + leafDER + noExts),
			},
			wantErr: "unknown entry type",
		},
		{
			leaf: LeafEntry{
				// version + leaf_type + timestamp + entry_type + key_hash + tbs + exts
				LeafInput: dh("00" + "00" + "0000015dcc997890" + "0001" + issuerKeyHash + precertTBS + noExts),
				ExtraData: dh("000508" + precertDER +
					("000ba3" + "0005cc" + precertCA + "0005d1" + precertRoot)),
			},
			wantPrecert: true,
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc997890" + "0001" + issuerKeyHash + corruptedPrecertTBS + noExts),
				ExtraData: dh("000508" + precertDER +
					("000ba3" + "0005cc" + precertCA + "0005d1" + precertRoot)),
			},
			wantErr: "failed to parse precertificate",
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc997890" + "0001" + issuerKeyHash + precertTBS + noExts),
				ExtraData: dh("000508" + precertDER +
					("000ba3" + "0005cc" + precertCA + "0005d1" + precertRoot) + "ff"),
			},
			wantErr: "PrecertChainEntry: trailing data",
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc997890" + "0001" + issuerKeyHash + precertTBS + noExts + "ff"),
				ExtraData: dh("000508" + precertDER +
					("000ba3" + "0005cc" + precertCA + "0005d1" + precertRoot)),
			},
			wantErr: "MerkleTreeLeaf: trailing data",
		},
		{
			leaf: LeafEntry{
				LeafInput: dh("00" + "00" + "0000015dcc997890" + "0001" + issuerKeyHash + precertTBS + noExts),
			},
			wantErr: "failed to unmarshal PrecertChainEntry",
		},
	}
	for i, test := range tests {
		got, err := LogEntryFromLeaf(int64(i), &test.leaf)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("LogEntryFromLeaf(%d) = _, %v; want _, nil", i, err)
			} else if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("LogEntryFromLeaf(%d) = _, %v; want _, err containing %q", i, err, test.wantErr)
			}
		} else if test.wantErr != "" {
			t.Errorf("LogEntryFromLeaf(%d) = _, nil; want _, err containing %q", i, test.wantErr)
		}
		if gotCert := (got != nil && got.X509Cert != nil); gotCert != test.wantCert {
			t.Errorf("LogEntryFromLeaf(%d).X509Cert = %v; want %v", i, gotCert, test.wantCert)
		}
		if gotPrecert := (got != nil && got.Precert != nil); gotPrecert != test.wantPrecert {
			t.Errorf("LogEntryFromLeaf(%d).Precert = %v; want %v", i, gotPrecert, test.wantPrecert)
		}
	}
}
