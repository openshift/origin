package main

import (
	"fmt"
	"net/http/httptest"
	"strings"
)

func main() {
	address := httptest.NewUnstartedServer(nil).Listener.Addr().String()
	parts := strings.Split(address, ":")
	fmt.Printf("%s\n", parts[1])
}
