package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime/serializer/json"
	"k8s.io/kubernetes/pkg/runtime/serializer/protobuf"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	_ "github.com/openshift/origin/pkg/quota/api/install"
	_ "k8s.io/kubernetes/pkg/api/install"
)

func main() {
	var endpoint, keyFile, certFile, caFile string
	flag.StringVar(&endpoint, "endpoint", "https://127.0.0.1:4001", "Etcd endpoint.")
	flag.StringVar(&keyFile, "key-file", "", "TLS client key.")
	flag.StringVar(&certFile, "cert-file", "", "TLS client certificate.")
	flag.StringVar(&caFile, "ca-file", "", "Server TLS CA certificate.")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: you need to specify action ls [<key>] or get <key>\n")
		os.Exit(1)
	}
	if flag.Arg(0) == "get" && flag.NArg() == 1 {
		fmt.Fprintf(os.Stderr, "ERROR: you need to specify <key> for get operation\n")
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
			CertFile: certFile,
			KeyFile:  keyFile,
			CAFile:   caFile,
		}
		var err error
		tlsConfig, err = tlsInfo.ClientConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: unable to create client config: %v\n", err)
			os.Exit(1)
		}
	}

	config := clientv3.Config{
		Endpoints:   []string{endpoint},
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
		resp, err = client.Get(context.TODO(), "/", clientv3.WithFromKey(), clientv3.WithKeysOnly())
	} else {
		resp, err = client.Get(context.TODO(), key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	}
	if err != nil {
		return err
	}
	for _, ev := range resp.Kvs {
		fmt.Printf("%s\n", ev.Key)
	}
	return nil
}

func getKey(client *clientv3.Client, key string) error {
	resp, err := client.Get(context.TODO(), key)
	if err != nil {
		return err
	}
	ps := protobuf.NewSerializer(api.Scheme, api.Scheme, "application/vnd.kubernetes.protobuf")
	js := json.NewSerializer(json.DefaultMetaFactory, api.Scheme, api.Scheme, true)
	for _, ev := range resp.Kvs {
		obj, gvk, err := ps.Decode(ev.Value, nil, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: unable to decode %s: %v\n", ev.Key, err)
			continue
		}
		fmt.Println(gvk)
		err = js.Encode(obj, os.Stdout)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: unable to decode %s: %v\n", ev.Key, err)
			continue
		}
	}
	return nil
}
