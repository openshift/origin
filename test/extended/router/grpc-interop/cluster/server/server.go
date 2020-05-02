package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/interop"

	testpb "google.golang.org/grpc/interop/grpc_testing"
)

const (
	defaultH2Port  = "8443"
	defaultH2CPort = "1110"
	defaultTLSCrt  = "/etc/service-certs/tls.crt"
	defaultTLSKey  = "/etc/service-certs/tls.key"
)

func lookupEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func main() {
	go func() {
		crt := lookupEnv("TLS_CRT", defaultTLSCrt)
		key := lookupEnv("TLS_KEY", defaultTLSKey)

		creds, err := credentials.NewServerTLSFromFile(crt, key)
		if err != nil {
			log.Fatalf("NewServerTLSFromFile failed: %v", err)
		}

		server := grpc.NewServer(grpc.Creds(creds))
		testpb.RegisterTestServiceServer(server, interop.NewTestServer())

		lis, err := net.Listen("tcp", ":"+lookupEnv("H2_PORT", defaultH2Port))
		if err != nil {
			log.Fatalf("listen failed: %v", err)
		}

		log.Printf("Serving h2 at: %v", lis.Addr())

		if err = server.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	go func() {
		server := grpc.NewServer()
		testpb.RegisterTestServiceServer(server, interop.NewTestServer())

		lis, err := net.Listen("tcp", ":"+lookupEnv("H2C_PORT", defaultH2CPort))
		if err != nil {
			log.Fatalf("listen failed: %v", err)
		}

		log.Printf("Serving h2c at: %v", lis.Addr())

		if err = server.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	select {}
}
