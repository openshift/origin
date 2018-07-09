[![GoDoc](https://godoc.org/github.com/bcicen/go-haproxy?status.svg)](https://godoc.org/github.com/bcicen/go-haproxy)
[![codebeat badge](https://codebeat.co/badges/f947c19e-0d7b-47d0-87b4-4e2e555ba806)](https://codebeat.co/projects/github-com-bcicen-go-haproxy)

# go-haproxy
Go library for interacting with HAProxys stats socket.

## Usage

Initialize a client object. Supported address schemas are `tcp://` and `unix:///`
```go
client := &haproxy.HAProxyClient{
  Addr: "tcp://localhost:9999",
}
```

Fetch results for a built in command(currently supports `show stats` and `show info`):
```go
stats, err := client.Stats()
for _, i := range stats {
	fmt.Printf("%s: %s\n", i.SvName, i.Status)
}
```

Or retrieve the result body from an arbitrary command string:
```go
result, err := h.RunCommand("show info")
fmt.Println(result.String())
```
