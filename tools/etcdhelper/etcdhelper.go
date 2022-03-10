package main

import (
	"bytes"
	"context"
    "crypto/aes"
    "crypto/cipher"
	"crypto/tls"
	"encoding/json"
    "encoding/base64"
	"flag"
	"fmt"
	"os"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	"go.etcd.io/etcd/client/v3"
	jsonserializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/kubectl/pkg/scheme"
    "k8s.io/apiserver/pkg/storage/value"
    aestransformer "k8s.io/apiserver/pkg/storage/value/encrypt/aes"
	"github.com/openshift/api"
)

const (
    AESCBC_PREFIX = "k8s:enc:aescbc:v1:"
)

func init() {
	api.Install(scheme.Scheme)
	api.InstallKube(scheme.Scheme)
}

func main() {
	var endpoints, keyFile, certFile, caFile, encryptionSecret, encryptionkey string
	flag.StringVar(&endpoints, "endpoints", "https://127.0.0.1:2379", "etcd endpoints.")
	flag.StringVar(&keyFile, "key", "", "TLS client key.")
	flag.StringVar(&certFile, "cert", "", "TLS client certificate.")
	flag.StringVar(&caFile, "cacert", "", "Server TLS CA certificate.")
    flag.StringVar(&encryptionkey, "encryption-key", getenv("ENCRYPTION_KEY"), "Encryption Key.")
    flag.StringVar(&encryptionSecret, "encryption-secret", getenv("ENCRYPTION_SECRET"), "Encryption Secret.")

	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprint(os.Stderr, "ERROR: you need to specify action: dump or ls [<key>] or get <key> or secrets <key>\n")
		os.Exit(1)
	}
	if flag.Arg(0) == "get" && flag.NArg() == 1 {
		fmt.Fprint(os.Stderr, "ERROR: you need to specify <key> for get operation\n")
		os.Exit(1)
	}
	if flag.Arg(0) == "dump" && flag.NArg() != 1 {
		fmt.Fprint(os.Stderr, "ERROR: you cannot specify positional arguments with dump\n")
		os.Exit(1)
	}
	action := flag.Arg(0)
	key := ""
	if flag.NArg() > 1 {
		key = flag.Arg(1)
	}

	var tlsConfig *tls.Config
	if len(certFile) != 0 || len(keyFile) != 0 || len(caFile) != 0 {
		tlsInfo := transport.TLSInfo{
			CertFile:      certFile,
			KeyFile:       keyFile,
			TrustedCAFile: caFile,
		}
		var err error
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: unable to create client config: %v\n", err)
			os.Exit(1)
		}
	}

	config := clientv3.Config{
		Endpoints:   []string{endpoints},
		TLS:         tlsConfig,
		DialTimeout: 5 * time.Second,
	}
	client, err := clientv3.New(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: unable to connect to etcd: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	switch action {
	case "ls":
		err = listKeys(client, key)
	case "get":
		err = getKey(client, key)
	case "dump":
		err = dump(client)
	case "secrets":
		err = secrets(encryptionkey, encryptionSecret, client, key)
	default:
		fmt.Fprintf(os.Stderr, "ERROR: invalid action: %s\n", action)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s-ing %s: %v\n", action, key, err)
		os.Exit(1)
	}
}

func listKeys(client *clientv3.Client, key string) error {
	var resp *clientv3.GetResponse
	var err error
	if len(key) == 0 {
		resp, err = clientv3.NewKV(client).Get(context.Background(), "/", clientv3.WithFromKey(), clientv3.WithKeysOnly())
	} else {
		resp, err = clientv3.NewKV(client).Get(context.Background(), key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	}
	if err != nil {
		return err
	}

	for _, kv := range resp.Kvs {
		fmt.Println(string(kv.Key))
	}

	return nil
}

func dump(client *clientv3.Client) error {
	response, err := clientv3.NewKV(client).Get(context.Background(), "/", clientv3.WithPrefix(), clientv3.WithSort(clientv3.SortByKey, clientv3.SortDescend))
	if err != nil {
		return err
	}

	kvData := []etcd3kv{}
	decoder := scheme.Codecs.UniversalDeserializer()
	encoder := jsonserializer.NewSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)
	objJSON := &bytes.Buffer{}

	for _, kv := range response.Kvs {
		obj, _, err := decoder.Decode(kv.Value, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: error decoding value %q: %v\n", string(kv.Value), err)
			continue
		}
		objJSON.Reset()
		if err := encoder.Encode(obj, objJSON); err != nil {
			fmt.Fprintf(os.Stderr, "WARN: error encoding object %#v as JSON: %v", obj, err)
			continue
		}
		kvData = append(
			kvData,
			etcd3kv{
				Key:            string(kv.Key),
				Value:          string(objJSON.Bytes()),
				CreateRevision: kv.CreateRevision,
				ModRevision:    kv.ModRevision,
				Version:        kv.Version,
				Lease:          kv.Lease,
			},
		)
	}

	jsonData, err := json.MarshalIndent(kvData, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))

	return nil
}

func getKey(client *clientv3.Client, key string) error {
	resp, err := clientv3.NewKV(client).Get(context.Background(), key)
	if err != nil {
		return err
	}

	decoder := scheme.Codecs.UniversalDeserializer()
    encoder := jsonserializer.NewYAMLSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	for _, kv := range resp.Kvs {
		obj, gvk, err := decoder.Decode(kv.Value, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: unable to decode %s: %v\n", kv.Key, err)
			continue
		}
		fmt.Println(gvk)
		err = encoder.Encode(obj, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: unable to encode %s: %v\n", kv.Key, err)
			continue
		}
	}

	return nil
}



func secrets(encryptionkey string, encryptionSecret string, client *clientv3.Client, key string) error {

	if len(encryptionkey) == 0 || len(encryptionSecret) == 0 {
		fmt.Fprint(os.Stderr, "ERROR: you need to specify an encryption key and secret\n")
		os.Exit(1)
	}

	response, err := clientv3.NewKV(client).Get(context.Background(), key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to Retrieve Key from ETCD: %v", err)
		os.Exit(1)
	}

	if !bytes.HasPrefix(response.Kvs[0].Value, []byte(AESCBC_PREFIX)) {
		fmt.Fprintf(os.Stderr, "Expected encrypted value to be prefixed with %s, but got %s", AESCBC_PREFIX, response.Kvs[0].Value)
		return err // Was only return
	}

	block, err := newAESCipher(encryptionSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create cipher from key: %v", err)
		os.Exit(1)

	}

	cbcTransformer := aestransformer.NewCBCTransformer(block)

	ctx := value.DefaultContext(key)

	enveloperPrefix := fmt.Sprintf("%s%s:", AESCBC_PREFIX, encryptionkey)

	sealedData := response.Kvs[0].Value[len(enveloperPrefix):]

	clearText, _, err := cbcTransformer.TransformFromStorage(sealedData, ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decrypt secret: %v", err)
		os.Exit(1)
	}

	decoder := scheme.Codecs.UniversalDeserializer()
	encoder := jsonserializer.NewYAMLSerializer(jsonserializer.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	obj, _, err := decoder.Decode(clearText, nil, nil)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to Decode: %v", err)
		os.Exit(1)
	}

	err = encoder.Encode(obj, os.Stdout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to decode to standard out: %v", err)
		os.Exit(1)

	}

	return nil
}

func newAESCipher(key string) (cipher.Block, error) {
	k, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return nil, fmt.Errorf("failed to decode config secret: %v", err)
	}

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %v", err)
	}

	return block, nil
}

func getenv(key string) string {

	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return ""
}

type etcd3kv struct {
	Key            string `json:"key,omitempty"`
	Value          string `json:"value,omitempty"`
	CreateRevision int64  `json:"create_revision,omitempty"`
	ModRevision    int64  `json:"mod_revision,omitempty"`
	Version        int64  `json:"version,omitempty"`
	Lease          int64  `json:"lease,omitempty"`
}
