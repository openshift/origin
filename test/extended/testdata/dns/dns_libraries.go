package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"
)

func main() {
	clusterIP := os.Getenv("DNS_CLUSTER_IP")
	if clusterIP == "" {
		log.Fatal("DNS_CLUSTER_IP must be set")
	}

	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 10 * time.Second,
			}
			return d.DialContext(ctx, network, net.JoinHostPort(clusterIP, "53"))
		},
	}
	_, err := r.LookupHost(context.Background(), "www.redhat.com")
	if err != nil {
		log.Fatalf("Failed to look up host: %v", err)
	}
}
