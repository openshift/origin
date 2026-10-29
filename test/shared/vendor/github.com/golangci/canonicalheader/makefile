.PHONY:

test:
	go test -v -race ./...

linter:
	golangci-lint -v run ./...

generate:
	go run ./cmd/initialismer/*.go -target="mapping" > ./initialism.go
	go run ./cmd/initialismer/*.go -target="test" > ./testdata/src/initialism/initialism.go
	go run ./cmd/initialismer/*.go -target="test-golden" > ./testdata/src/initialism/initialism.go.golden
	gofmt -w ./initialism.go ./testdata/src/initialism/initialism.go ./testdata/src/initialism/initialism.go.golden
