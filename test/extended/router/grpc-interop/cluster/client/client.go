package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/interop"
	"google.golang.org/grpc/resolver"

	testpb "google.golang.org/grpc/interop/grpc_testing"
)

type testFn func(tc testpb.TestServiceClient, args ...grpc.CallOption)

var defaultTestCases = map[string]testFn{
	"cancel_after_begin":          interop.DoCancelAfterBegin,
	"cancel_after_first_response": interop.DoCancelAfterFirstResponse,
	"client_streaming":            interop.DoClientStreaming,
	"custom_metadata":             interop.DoCustomMetadata,
	"empty_unary":                 interop.DoEmptyUnaryCall,
	"large_unary":                 interop.DoLargeUnaryCall,
	"ping_pong":                   interop.DoPingPong,
	"server_streaming":            interop.DoServerStreaming,
	"special_status_message":      interop.DoSpecialStatusMessage,
	"status_code_and_message":     interop.DoStatusCodeAndMessage,
	"timeout_on_sleeping_server":  interop.DoTimeoutOnSleepingServer,
	"unimplemented_method":        nil, // special case
	"unimplemented_service":       nil, // special case
}

var (
	listTests = flag.Bool("list-tests", false, "List available test cases")
	insecure  = flag.Bool("insecure", false, "Skip certificate verification")
	caFile    = flag.String("ca-cert", "", "The file containing the CA root cert")
	useTLS    = flag.Bool("tls", false, "Connection uses TLS, if true")
	host      = flag.String("host", "localhost", "host address")
	port      = flag.Int("port", 443, "port number")
	count     = flag.Int("count", 1, "Run each test case N times")
)

type DialParams struct {
	UseTLS   bool
	CertData []byte
	Host     string
	Port     int
	Insecure bool
}

func Dial(cfg DialParams) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	if cfg.UseTLS {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: cfg.Insecure,
		}
		if len(cfg.CertData) > 0 {
			rootCAs, err := x509.SystemCertPool()
			if err != nil {
				return nil, err
			}
			if rootCAs == nil {
				rootCAs = x509.NewCertPool()
			}
			if ok := rootCAs.AppendCertsFromPEM(cfg.CertData); !ok {
				return nil, errors.New("failed to append certs")
			}
			tlsConfig.RootCAs = rootCAs
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	return grpc.Dial(net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), append(opts, grpc.WithBlock())...)
}

func main() {
	flag.Parse()

	if *listTests {
		for k := range defaultTestCases {
			fmt.Println(k)
		}
		os.Exit(0)
	}

	dialParams := DialParams{
		UseTLS:   *useTLS,
		Host:     *host,
		Port:     *port,
		Insecure: *insecure,
	}

	if *caFile != "" {
		certs, err := ioutil.ReadFile(*caFile)
		if err != nil {
			log.Fatalf("Failed to read %q: %v", *caFile, err)
		}
		dialParams.CertData = certs
	}

	testCases := flag.Args()
	if len(testCases) == 0 {
		for k := range defaultTestCases {
			testCases = append(testCases, k)
		}
	}

	resolver.SetDefaultScheme("dns")
	conn, err := Dial(dialParams)
	if err != nil {
		log.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	for i := 0; i < *count; i++ {
		for _, testCase := range testCases {
			log.Printf("[%v/%v] running test case %q\n", i+1, *count, testCase)
			if doRpcCall, ok := defaultTestCases[testCase]; ok && doRpcCall != nil {
				doRpcCall(testpb.NewTestServiceClient(conn))
			} else if ok && doRpcCall == nil {
				switch testCase {
				case "unimplemented_method":
					interop.DoUnimplementedMethod(conn)
				case "unimplemented_service":
					interop.DoUnimplementedService(testpb.NewUnimplementedServiceClient(conn))
				default:
					log.Fatalf("Unsupported test case: %q", testCase)
				}
			} else {
				log.Fatalf("Unsupported test case: %q", testCase)
			}
		}
	}
}
