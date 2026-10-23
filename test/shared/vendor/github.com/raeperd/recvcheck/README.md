# recvcheck

[![Build Status](https://github.com/raeperd/recvcheck/actions/workflows/build.yaml/badge.svg)](https://github.com/raeperd/recvcheck/actions/workflows/build.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/raeperd/recvcheck)](https://goreportcard.com/report/github.com/raeperd/recvcheck)
[![Coverage Status](https://coveralls.io/repos/github/raeperd/recvcheck/badge.svg?branch=main)](https://coveralls.io/github/raeperd/recvcheck?branch=main)
[![Go Reference](https://pkg.go.dev/badge/github.com/raeperd/recvcheck.svg)](https://pkg.go.dev/github.com/raeperd/recvcheck)

Go linter that detects mixing pointer and value method receivers.

## Why

Mixing pointer and value receivers on the same type creates **subtle, hard-to-detect bugs** that can cause:

### 🐛 Data Races
When you copy a struct with a mutex, you copy the mutex state, leading to race conditions:

```go
type RPC struct {
    mu     sync.Mutex  // This is the problem!
    result int
    done   chan struct{}
}

func (rpc *RPC) compute() {
    rpc.mu.Lock()
    defer rpc.mu.Unlock()
    rpc.result = 42
}

func (RPC) version() int {  // Value receiver copies the mutex!
    return 1
}

func main() {
    rpc := &RPC{done: make(chan struct{})}
    go rpc.compute()         // Locks original mutex
    version := rpc.version() // Uses copied mutex - RACE!
    // ...
}
```

### 🚨 Silent Bugs

Value receivers create copies, so modifications are lost:

```go
type Counter struct {
    value int
}

func (c *Counter) Increment() { c.value++ }  // pointer receiver
func (c Counter) Reset() { c.value = 0 }     // value receiver - NO EFFECT!
```

### 🤔 Developer Confusion

Mixed receivers make code behavior unpredictable and harder to reason about.

### ✅ The Solution

**Consistency is key**: Go's official guidance says [Don't mix receiver types](https://go.dev/wiki/CodeReviewComments#receiver-type). Choose either pointers or values for **all** methods on a type.

`recvcheck` automatically detects these issues before they reach production.

## Installation

```bash
# Standalone
go install github.com/raeperd/recvcheck/cmd/recvcheck@latest

# With golangci-lint (recommended)
# Add to .golangci.yml:
linters:
  enable:
    - recvcheck
```

## Usage

```bash
recvcheck ./...
# or
golangci-lint run
```

Output:
```
main.go:8:1: the methods of "RPC" use pointer receiver and non-pointer receiver
```

## Configuration

```yaml
# .golangci.yml
linters-settings:
  recvcheck:
    # Disable default exclusions (MarshalJSON, etc.)
    disable-builtin: false
    
    # Custom exclusions
    exclusions:
      - "Server.Shutdown"   # Specific method
      - "*.String"          # All String methods
```

### Default Exclusions

Unmarshal methods are excluded by default as they must use pointer receivers:
- `*.UnmarshalText`, `*.UnmarshalJSON`, `*.UnmarshalYAML`
- `*.UnmarshalXML`, `*.UnmarshalBinary`, `*.GobDecode`

## Examples

❌ **Bad** - Mixed receivers:
```go
func (u *User) SetName(name string) { }  // pointer
func (u User) GetName() string { }       // value - inconsistent!
```

✅ **Good** - Consistent receivers:
```go
func (u *User) SetName(name string) { }  // pointer
func (u *User) GetName() string { }      // pointer - consistent!
```

## Contributing

```bash
make test  # Run tests
make lint  # Run linter
make build # Build binary
```

## License

MIT